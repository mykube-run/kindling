package types

type Event struct {
	Md5  string
	Data []byte
}

type ConfigSource interface {
	Read() ([]byte, error)
	Watch() (<-chan Event, error)
	Close() error
}
