package main

import (
	"flag"
	"fmt"
	"github.com/mykube-run/kindling/pkg/kconfig"
	"github.com/mykube-run/kindling/pkg/types"
	"log"
)

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

type ExampleConfig struct {
	IntVal int `mapstructure:"int"`
}

var (
	hdl = types.ConfigUpdateHandler{
		Name: "example",
		Handle: func(prev, cur interface{}) error {
			fmt.Printf("previous config: %+v -> ", prev)
			fmt.Printf("current config: %+v\n", cur)
			return nil
		},
	}
	custom = flag.Int("custom", 0, "User application custom config")
)

func main() {
	_ = custom
	_, err := kconfig.New(Proxy, hdl)
	if err != nil {
		log.Fatalf("failed to create new config manager: %v", err)
	}
	fmt.Println(Proxy.Get().(ExampleConfig).IntVal, Proxy.Value().IntVal)
}
