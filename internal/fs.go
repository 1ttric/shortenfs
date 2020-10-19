package internal

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	_ "bazil.org/fuse/fs/fstestutil"
	"context"
	"github.com/1ttric/shortenfs/internal/config"
	"github.com/1ttric/shortenfs/internal/drivers"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

var (
	shortenBlock *ShortenBlock
)

func Mount(mountpoint string, driver drivers.Driver) {
	shortenBlock = NewShortenBlock(driver, config.MainConfig)

	// Unmount in case of a previous dirty exit
	_ = fuse.Unmount(mountpoint)

	// Mount
	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("shortenfs"),
		fuse.Subtype("shortenfs"),
	)
	if err != nil {
		log.Fatal(err)
	}
	sigs := make(chan os.Signal)
	done := make(chan struct{})
	signal.Notify(sigs, syscall.SIGINT)
	go func() {
		<-sigs
		log.Infof("unmounting filesystem")
		_ = fuse.Unmount(mountpoint)
		done <- struct{}{}
	}()
	go func() {
		_ = fs.Serve(c, FS{})
		done <- struct{}{}
	}()
	log.Infof("mounted filesystem")
	<-done

	// Update config with new root ID before exiting
	config.MainConfig.RootID = shortenBlock.GetRootID()
	log.Infof("saving configuration")
	config.Write()
}

type FS struct{}

func (FS) Root() (fs.Node, error) {
	return Dir{}, nil
}

type Dir struct{}

func (Dir) Attr(_ context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Uid = 0
	a.Gid = 0
	a.Mode = os.ModeDir | 0o777
	return nil
}

func (Dir) Lookup(_ context.Context, name string) (fs.Node, error) {
	if name == "block" {
		return &File{}, nil
	}
	return nil, syscall.ENOENT
}

var dirFiles = []fuse.Dirent{
	{Inode: 2, Name: "block", Type: fuse.DT_File},
}

func (Dir) ReadDirAll(_ context.Context) ([]fuse.Dirent, error) {
	return dirFiles, nil
}

type File struct{}

func (f *File) Attr(_ context.Context, a *fuse.Attr) error {
	log.Trace("attr")
	a.Inode = 2
	a.Gid = 0
	a.Uid = 0
	a.Mode = 0o777
	a.Size = uint64(shortenBlock.Capacity())
	return nil
}

func (f *File) Read(_ context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	log.Tracef("read %d, %d", req.Size, req.Offset)
	data, err := shortenBlock.Read(req.Size, int(req.Offset))
	if err != nil {
		return err
	}
	resp.Data = data
	return nil
}

func (f *File) Write(_ context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	log.Tracef("write %d, %d", req.Offset, len(req.Data))
	n, err := shortenBlock.Write(int(req.Offset), req.Data)
	if err != nil {
		return err
	}
	resp.Size = n
	return nil
}

func (f *File) Fsync(_ context.Context, _ *fuse.FsyncRequest) error {
	log.Trace("fsync")
	return nil
}
