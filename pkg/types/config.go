package types

// ConfigProxy is a proxy for user's business config, defining the way
// user's config is accessed & modified by kconfig.Manager, as well as
// how to generate a brand new config proxy.
type ConfigProxy interface {
	Get() interface{}
	Populate(func(interface{}) error) error
	New() ConfigProxy
}

// ConfigUpdateHandler is called when config change, it enables user to compare
// the new config with previous one, and decide what kind of action should be taken, e.g.
// reconnect database, refresh cache or send a notification.
type ConfigUpdateHandler struct {
	Name   string
	Handle func(prev, cur interface{}) error
}
