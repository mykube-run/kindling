package kconfig

import (
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/mykube-run/kindling/pkg/types"
	"github.com/mykube-run/kindling/pkg/utils"
	"gopkg.in/yaml.v3"
	"time"
)

type Manager struct {
	opt      *BootstrapOption
	src      types.ConfigSource
	proxy    types.ConfigProxy
	handlers []types.ConfigUpdateHandler
	lg       types.Logger

	unmarshalFn func([]byte, interface{}) error
	lastUpdate  time.Time
	lastMd5     string
}

// New creates a new Manager instance, which will automatically read BootstrapOption from environment & flags.
// Once created, Manager will read config data and update config with the help of types.ConfigProxy.
// hdl is a series of custom update handlers, will be called sequentially after Manager is created and when config is changed.
func New(proxy types.ConfigProxy, hdl ...types.ConfigUpdateHandler) (*Manager, error) {
	opt := NewBootstrapOptionFromEnvFlag()
	return NewWithOption(proxy, opt, hdl...)
}

// NewWithOption creates a new Manager instance with given BootstrapOption.
func NewWithOption(proxy types.ConfigProxy, opt *BootstrapOption, hdl ...types.ConfigUpdateHandler) (*Manager, error) {
	if err := validateParams(proxy, opt); err != nil {
		return nil, err
	}
	src, err := NewConfigSource(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create config source: %w", err)
	}

	m := newManager(proxy, opt, src, hdl...)
	if err = m.readAndUpdate(); err != nil {
		return nil, err
	}
	return m, m.watch()
}

// Register registers extra event handlers after creation
func (m *Manager) Register(hdl ...types.ConfigUpdateHandler) *Manager {
	m.handlers = append(m.handlers, hdl...)
	return m
}

// readAndUpdate is called after Manager is created
func (m *Manager) readAndUpdate() error {
	byt, err := m.src.Read()
	if err != nil {
		return fmt.Errorf("error reading config: %w", err)
	}
	evt := types.Event{
		Md5:  utils.Md5(byt),
		Data: byt,
	}
	return m.onUpdate(evt)
}

// onUpdate handles config update event
func (m *Manager) onUpdate(evt types.Event) error {
	// Compare md5 and update time
	if m.lastMd5 == evt.Md5 || evt.Data == nil {
		m.lg.Trace("config was not changed and will be ignored (having the same md5 or was nil)")
		return nil
	}
	if m.lastUpdate.Add(m.opt.MinimalInterval).After(time.Now()) {
		m.lg.Warn("config was changed not long ago and will be ignored")
		return nil
	}

	// Construct a config populate closure function
	fn, err := m.populateFunc(evt.Data)
	if err != nil {
		return fmt.Errorf("error creating config populate function: %w", err)
	}

	// Create a new proxy and populate it
	cur := m.proxy.New()
	if err = cur.Populate(fn); err != nil {
		return fmt.Errorf("error populating new config: %w", err)
	}

	// Handle config change
	for _, hdl := range m.handlers {
		if err := hdl.Handle(m.proxy.Get(), cur.Get()); err != nil {
			return fmt.Errorf("handler [%s] failed: %w", hdl.Name, err)
		}
		m.lg.Trace(fmt.Sprintf("handler [%s] finished", hdl.Name))
	}

	// Populate the new config back to original config
	if err = m.proxy.Populate(fn); err != nil {
		return fmt.Errorf("error populating config: %w", err)
	}
	m.lastUpdate = time.Now()
	m.lastMd5 = evt.Md5
	m.lg.Info(fmt.Sprintf("updated config, md5: %v", m.lastMd5))
	return nil
}

func (m *Manager) populateFunc(byt []byte) (func(interface{}) error, error) {
	// Unmarshal config bytes into a temporary map, which will be used by mapstructure decoder later
	var tmp map[string]interface{}
	if err := m.unmarshalFn(byt, &tmp); err != nil {
		return nil, err
	}

	fn := func(v interface{}) error {
		dc := &mapstructure.DecoderConfig{
			WeaklyTypedInput: true,
			ZeroFields:       true, // this must be set to avoid array/map being merged
			Result:           v,
		}
		decoder, err := mapstructure.NewDecoder(dc)
		if err != nil {
			return err
		}
		return decoder.Decode(tmp)
	}
	return fn, nil
}

func (m *Manager) watch() error {
	eventC, err := m.src.Watch()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case evt, ok := <-eventC:
				if !ok {
					m.lg.Trace("config manager closed, stop watching")
					return
				}
				if e := m.onUpdate(evt); e != nil {
					m.lg.Error(fmt.Sprintf("update config failed, md5: %v, error: %s", evt.Md5, err))
				}
			}
		}
	}()
	return nil
}

func validateParams(proxy types.ConfigProxy, opt *BootstrapOption) error {
	if proxy.Get() == nil {
		return fmt.Errorf("config proxy should always return a valid config")
	}
	if opt == nil {
		return fmt.Errorf("bootstrap option must be provided")
	}
	if err := opt.Validate(); err != nil {
		return fmt.Errorf("invalid bootstrap option: %w", err)
	}
	return nil
}

func newManager(proxy types.ConfigProxy, opt *BootstrapOption, src types.ConfigSource, hdl ...types.ConfigUpdateHandler,
) *Manager {
	m := &Manager{
		opt:      opt,
		src:      src,
		proxy:    proxy,
		handlers: hdl,
		lg:       opt.Logger,
	}
	switch opt.Format {
	case "json":
		m.unmarshalFn = json.Unmarshal
	case "yaml":
		m.unmarshalFn = yaml.Unmarshal
	}
	return m
}
