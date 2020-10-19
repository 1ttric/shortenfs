package internal

import (
	"bytes"
	"github.com/1ttric/shortenfs/internal/config"
	"github.com/1ttric/shortenfs/internal/drivers"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"math"
	"strings"
	"time"
)

var (
	// A read cache is used so that every single read doesn't need a complete HTTP roundtrip
	readCache = cache.New(5*time.Minute, 10*time.Minute)
)

// Used to store the filesystem node tree - parent short IDs can contain multiple child short IDs, with leaf nodes
// pointing to chunks of actual data
type Node struct {
	id       string
	parent   *Node
	children []*Node
}

type ShortenBlock struct {
	// Depth of the tree
	depth int
	// Stores the top-level node of shortened data
	tree *Node
	// The actual shortener implementation to use (tinyurl, bitly, etc)
	shortener drivers.Driver

	// Number of child node IDs per parent node, accounting for comma separators
	idsPerNode int
}

func NewShortenBlock(shortener drivers.Driver, config config.ShortenBlockConfig) *ShortenBlock {
	if config.Depth <= 0 {
		log.Fatalf("invalid depth")
	}
	if config.RootID == "" {
		log.Debugf("no defined root - creating new filesystem")
	}
	return &ShortenBlock{
		depth:      config.Depth,
		tree:       &Node{id: config.RootID},
		shortener:  shortener,
		idsPerNode: (shortener.NodeSize() + 1) / (shortener.IdSize() + 1),
	}
}

// Fetches the leaf node indexed by leafIdx
func (s *ShortenBlock) getLeaf(leafIdx int) (*Node, error) {
	// Plots the child index for each node of the tree we need to visit to get to the final requested leaf
	var path []int
	for level := 0; level < s.depth; level++ {
		childIdx := (leafIdx / int(math.Pow(float64(s.idsPerNode), float64(level)))) % s.idsPerNode
		path = append([]int{childIdx}, path...)
	}

	log.Tracef("traversing path %v to leaf idx %d", path, leafIdx)
	node := s.tree
	for _, childIdx := range path {
		// Perform lazy initialization for nodes which have not been written to
		if len(node.children) == 0 {
			if node.id != "" {
				// If a node is named, load its children from the shortener
				var data []byte
				var err error
				if data, err = s.cachedNodeRead(node.id); err != nil {
					return nil, err
				}
				log.Tracef("node %s children are %s", node.id, string(data))
				for _, childID := range strings.Split(strings.Trim(string(data), "\x00"), ",") {
					node.children = append(node.children, &Node{id: childID, parent: node})
				}
			} else {
				// If a node has no name (is heretofore unwritten), create empty children for it
				for i := 0; i < s.idsPerNode; i++ {
					node.children = append(node.children, &Node{parent: node})
				}
			}
		}

		log.Tracef("node %s chose child idx %d: %s", node.id, childIdx, node.children[childIdx].id)
		node = node.children[childIdx]
	}

	return node, nil
}

// Read data from a node, but with a cache - this means reads do not require an entire HTTP roundtrip
func (s *ShortenBlock) cachedNodeRead(id string) ([]byte, error) {
	cachedData, ok := readCache.Get(id)
	if ok {
		log.Debugf("cache hit for id %s", id)
		return cachedData.([]byte), nil
	}
	log.Debugf("reading %s", id)
	data, err := s.shortener.Read(id)
	if err != nil {
		return nil, err
	}
	log.Debugf("read %d from %s", len(data), id)
	readCache.SetDefault(id, data)
	return data, nil
}

// Writes the specified data to a node and updates the resulting short ID
// Then, updates the parent node's data with the new short ID
// This is performed recursively up to the root node
func (s *ShortenBlock) nodeWrite(node *Node, data []byte) error {
	var newID string
	var err error
	log.Debugf("writing %d bytes to node", len(data))
	if newID, err = s.shortener.Write(data); err != nil {
		return err
	}
	log.Tracef("node id changed from %s to %s", node.id, newID)

	node.id = newID
	for {
		node = node.parent
		log.Tracef("updating parent node %s", node.id)

		var childIDs []string
		for _, child := range node.children {
			childIDs = append(childIDs, child.id)
		}
		newData := strings.Join(childIDs, ",")
		log.Tracef("new child nodes are %s", newData)

		var newID string
		log.Debugf("writing %d bytes to node parent", len(newData))
		if newID, err = s.shortener.Write([]byte(newData)); err != nil {
			return err
		}
		node.id = newID
		for _, child := range node.children {
			child.parent = node
		}
		if node.parent == nil {
			break
		}
	}
	return nil
}

// Returns the capacity, in raw bytes, of the filesystem
func (s *ShortenBlock) Capacity() int {
	return int(math.Pow(float64(s.idsPerNode), float64(s.depth))) * s.shortener.NodeSize()
}

// Presenting leaf nodes as a contiguous chunk, reads a chunk of the given size at a given offset
func (s *ShortenBlock) Read(size int, offset int) ([]byte, error) {
	log.Debugf("reading %d bytes at offset %d", size, offset)
	// Determine which leaves will need to be accessed in order to satisfy the requested read
	startLeafIdx := offset / s.shortener.NodeSize()
	endLeafIdx := int(math.Ceil(float64(offset+size) / float64(s.shortener.NodeSize())))

	var readData []byte
	for leafIdx := startLeafIdx; leafIdx < endLeafIdx; leafIdx++ {
		var leaf *Node
		var err error
		log.Debugf("reading from leaf %d of (%d, %d)", leafIdx, startLeafIdx, endLeafIdx)

		if leaf, err = s.getLeaf(leafIdx); err != nil {
			log.Debugf("error getting leaf %d: %s", leafIdx, err.Error())
			return []byte{}, err
		}

		// For this leaf, calculate the subread start and end indices
		var subReadStart int
		var subReadEnd int
		if leafIdx == startLeafIdx {
			subReadStart = offset % s.shortener.NodeSize()
		} else {
			subReadStart = 0
		}
		if leafIdx == endLeafIdx-1 {
			// Edge case where readEnd is on the end border of the chunk
			//log.Trace((offset + size) % NodeSize)
			if (offset+size)%s.shortener.NodeSize() == 0 {
				subReadEnd = s.shortener.NodeSize()
			} else {
				subReadEnd = (offset + size) % s.shortener.NodeSize()
			}
		} else {
			subReadEnd = s.shortener.NodeSize()
		}
		log.Tracef("leaf subread indices: (%d, %d)", subReadStart, subReadEnd)

		var leafData []byte
		if leaf.id != "" {
			if leafData, err = s.cachedNodeRead(leaf.id); err != nil {
				return []byte{}, err
			}
		} else {
			leafData = bytes.Repeat([]byte{0}, s.shortener.NodeSize())
		}
		leafData = append(leafData, bytes.Repeat([]byte{0}, s.shortener.NodeSize()-len(leafData))...)
		subReadData := leafData[subReadStart:subReadEnd]

		readData = append(readData, subReadData...)
	}
	return readData, nil
}

// Presenting leaf nodes as a contiguous chunk, writes the given data at the given offset
func (s *ShortenBlock) Write(offset int, data []byte) (int, error) {
	size := len(data)
	log.Debugf("writing %d bytes at offset %d", size, offset)
	startLeafIdx := offset / s.shortener.NodeSize()
	endLeafIdx := int(math.Ceil(float64(offset+size) / float64(s.shortener.NodeSize())))

	bytesWritten := 0

	for leafIdx := startLeafIdx; leafIdx < endLeafIdx; leafIdx++ {
		log.Debugf("writing to leaf %d of range (%d, %d)", leafIdx, startLeafIdx, endLeafIdx)
		var leaf *Node
		var err error
		if leaf, err = s.getLeaf(int(leafIdx)); err != nil {
			log.Debugf("could not retrieve leaf: %s", err.Error())
			return 0, nil
		}

		var subWriteStart int
		var subWriteEnd int
		if leafIdx == startLeafIdx {
			subWriteStart = offset % s.shortener.NodeSize()
		} else {
			subWriteStart = 0
		}
		if leafIdx == endLeafIdx-1 {
			subWriteEnd = (offset + size) % s.shortener.NodeSize()
		} else {
			subWriteEnd = s.shortener.NodeSize()
		}

		var leafData []byte
		if leaf.id != "" {
			if leafData, err = s.cachedNodeRead(leaf.id); err != nil {
				log.Errorf("could not read leaf data: %s", err.Error())
				return 0, err
			}
		} else {
			leafData = bytes.Repeat([]byte{0}, s.shortener.NodeSize())
		}
		log.Tracef("concatenating old leaf data of length %d with range (%d, %d)", len(leafData), subWriteStart, subWriteEnd)
		newLeafData := leafData[:subWriteStart]
		newLeafData = append(newLeafData, data[:subWriteEnd-subWriteStart]...)
		newLeafData = append(newLeafData, leafData[int(math.Min(float64(subWriteEnd), float64(len(leafData)))):]...)
		data = data[subWriteEnd-subWriteStart:]

		err = s.nodeWrite(leaf, newLeafData)
		if err != nil {
			log.Errorf("fs.nodeWrite returned error: %s", err.Error())
		}
		bytesWritten += subWriteEnd - subWriteStart
	}

	return bytesWritten, nil
}

// Returns the root shortlink of the filesystem
func (s *ShortenBlock) GetRootID() string {
	return s.tree.id
}
