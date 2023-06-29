package kconfig

// ConfigProxy is a proxy for user's business config, defining the way
// user's config is accessed & modified by kconfig.Manager, as well as
// how to generate a brand-new config proxy.
//
// When configuration was updated, a kconfig manager will:
// 1) Create a new temporary proxy using ConfigProxy.New
// 2) Wrap new config data (in bytes) in a closure function, then calls ConfigProxy.Populate
// 3) Call update handlers one by one
// 4) Call manager.ConfigProxy.Populate to update existing config
type ConfigProxy interface {
	// Get always returns current config value holding by ConfigProxy
	Get() interface{}
	// Populate takes a closure function as input, fills new config data into its config value
	Populate(func(interface{}) error) error
	// New creates a ConfigProxy with an empty config instance
	New() ConfigProxy
}

// ConfigUpdateHandler is called when config change, it enables user to compare
// the new config with previous one, and decide what kind of action should be taken, e.g.
// reconnect database, refresh cache or send a notification.
type ConfigUpdateHandler struct {
	Name   string
	Handle func(prev, cur interface{}) error
}

// NOOPHandler does nothing on config update
var NOOPHandler = ConfigUpdateHandler{
	Name: "noop",
	Handle: func(prev, cur interface{}) error {
		return nil
	},
}
