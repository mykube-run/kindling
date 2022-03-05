package main

import "github.com/mykube-run/kindling/pkg/types"

var Proxy = &proxy{c: new(ExampleConfig)}

type proxy struct {
	c *ExampleConfig
}

func (p *proxy) Get() interface{} {
	return *p.c
}

func (p *proxy) Populate(fn func(interface{}) error) error {
	return fn(p.c)
}

func (p *proxy) New() types.ConfigProxy {
	return &proxy{c: new(ExampleConfig)}
}

func (p *proxy) Value() ExampleConfig {
	return *p.c
}

func IntVal() int {
	return Proxy.Get().(ExampleConfig).IntVal
}

type ExampleConfig struct {
	IntVal int `mapstructure:"int"`
}
