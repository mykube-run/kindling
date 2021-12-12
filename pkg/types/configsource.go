package types

// ConfigSource is the underlying config source for kconfig, responsible for
// reading config data and watching changes.
type ConfigSource interface {
	Read() ([]byte, error)
	Watch() (<-chan Event, error)
	Close() error
}

// ConfigSourceType specifies config sources that kconfig currently supports.
type ConfigSourceType string

const (
	File   ConfigSourceType = "file" // file can be json, yaml
	Etcd   ConfigSourceType = "etcd" // etcd v3
	Consul ConfigSourceType = "consul"
	Nacos  ConfigSourceType = "nacos"
)

// Event represents a config update event. Md5 can be used to filter repeat events.
type Event struct {
	Md5  string
	Data []byte
}
