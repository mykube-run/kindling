package kconfig

import (
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/mykube-run/kindling/pkg/types"
	"github.com/mykube-run/kindling/pkg/utils"
	"gopkg.in/yaml.v3"
	"reflect"
	"time"
)

type ConfigEventHandler struct {
	Name   string
	Handle func(old, new interface{}) error
}

type Manager struct {
	opt      *BootstrapOption
	src      types.ConfigSource
	conf     interface{}
	newConf  func() interface{}
	handlers []ConfigEventHandler
	lg       types.Logger

	unmarshalFn func([]byte, interface{}) error
	lastUpdate  time.Time
	lastMd5     string
}

// New creates a new Manager instance, which will automatically read BootstrapOption from environment & flags.
// Once created, Manager will read config source and update given config instance.
// conf must be a non nil pointer to the config, any changes will be populated back.
// newConf must returns a non nil config pointer.
// hdl is a series of custom update handlers, will be called sequentially after Manager is created and when config is changed.
func New(conf interface{}, newConf func() interface{}, hdl ...ConfigEventHandler) (*Manager, error) {
	opt := NewBootstrapOptionFromEnvFlag()
	return NewWithOption(conf, newConf, opt, hdl...)
}

// NewWithOption creates a new Manager instance with given BootstrapOption.
func NewWithOption(conf interface{}, newConf func() interface{}, opt *BootstrapOption, hdl ...ConfigEventHandler) (*Manager, error) {
	if err := validateParams(conf, newConf, opt); err != nil {
		return nil, err
	}
	src, err := NewConfigSource(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create config source: %w", err)
	}

	m := newManager(conf, newConf, opt, src, hdl...)
	if err = m.readAndUpdate(); err != nil {
		return nil, err
	}
	return m, m.watch()
}

// Register registers extra event handlers after creation
func (m *Manager) Register(hdl ...ConfigEventHandler) *Manager {
	m.handlers = append(m.handlers, hdl...)
	return m
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

	// Read in new config
	conf := m.newConf()
	if err := m.unmarshal(evt.Data, conf); err != nil {
		return fmt.Errorf("error unmarshalling new config: %w", err)
	}

	// Handle config change
	for _, hdl := range m.handlers {
		if err := hdl.Handle(m.conf, conf); err != nil {
			return fmt.Errorf("handler [%s] failed: %w", hdl.Name, err)
		}
		m.lg.Trace(fmt.Sprintf("handler [%s] finished", hdl.Name))
	}

	// Populate the new config back to original config
	if err := m.unmarshal(evt.Data, m.conf); err != nil {
		return fmt.Errorf("error unmarshalling new config: %w", err)
	}
	m.lastUpdate = time.Now()
	m.lastMd5 = evt.Md5
	m.lg.Info(fmt.Sprintf("updated config, md5: %v", m.lastMd5))
	return nil
}

func (m *Manager) unmarshal(byt []byte, v interface{}) error {
	var tmp map[string]interface{}
	if err := m.unmarshalFn(byt, &tmp); err != nil {
		return err
	}

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

func validateParams(conf interface{}, newConf func() interface{}, opt *BootstrapOption) error {
	if conf == nil || reflect.TypeOf(conf).Kind() != reflect.Ptr {
		return fmt.Errorf("config must be a non nil pointer")
	}
	tmp := newConf()
	if tmp == nil || reflect.TypeOf(tmp).Kind() != reflect.Ptr {
		return fmt.Errorf("newConf must return a non nil pointer")
	}
	if opt == nil {
		return fmt.Errorf("bootstrap option must be provided")
	}
	if err := opt.Validate(); err != nil {
		return fmt.Errorf("invalid bootstrap option: %w", err)
	}
	return nil
}

func newManager(conf interface{}, newConf func() interface{},
	opt *BootstrapOption, src types.ConfigSource, hdl ...ConfigEventHandler,
) *Manager {
	m := &Manager{
		opt:      opt,
		src:      src,
		conf:     conf,
		newConf:  newConf,
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
