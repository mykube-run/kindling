package main

import (
	"flag"
	"fmt"
	"github.com/mykube-run/kindling/pkg/kconfig"
	"log"
)

type ExampleConfig struct {
	IntVal int `mapstructure:"int"`
}

var (
	c       = new(ExampleConfig)
	newConf = func() interface{} {
		return new(ExampleConfig)
	}
	snapshot = func() ExampleConfig {
		return *c
	}
	hdl = kconfig.ConfigEventHandler{
		Name: "example",
		Handle: func(old, new interface{}) error {
			fmt.Println(old)
			fmt.Println(new)
			return nil
		},
	}
	custom = flag.Int("custom", 0, "User application custom config")
)

func main() {
	_ = custom
	_, err := kconfig.New(c, newConf, hdl)
	if err != nil {
		log.Fatalf("failed to create new config manager: %v", err)
	}
	fmt.Println(snapshot().IntVal)
}
