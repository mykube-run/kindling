package config

import (
	"github.com/mykube-run/kindling/pkg/kconfig"
)

// Proxy kconfig proxy
// --------------------
var Proxy = &proxy{c: new(Sample)}

type proxy struct {
	c *Sample
}

func (p *proxy) Get() interface{} {
	return *p.c
}

func (p *proxy) Populate(fn func(interface{}) error) error {
	return fn(p.c)
}

func (p *proxy) New() kconfig.ConfigProxy {
	return &proxy{c: new(Sample)}
}
