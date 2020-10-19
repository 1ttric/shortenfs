package drivers

var (
	drivers = make(map[string]Driver)
)

func Register(name string, driver Driver) {
	drivers[name] = driver
}

func Get(name string) (Driver, bool) {
	driver, ok := drivers[name]
	return driver, ok
}

type Driver interface {
	// Returns the number of storable bytes in one shortlink
	NodeSize() int
	// Returns shortlink ID size
	IdSize() int
	// Read data from a shortlink ID
	Read(shortId string) (data []byte, err error)
	// Write data and return a shortlink
	Write(data []byte) (shortId string, err error)
}
