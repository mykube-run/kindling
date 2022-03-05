package main

import (
	"flag"
	"fmt"
	"github.com/mykube-run/kindling/pkg/kconfig"
	"github.com/mykube-run/kindling/pkg/types"
	"log"
	"time"
)

var (
	hdl = types.ConfigUpdateHandler{
		Name: "example",
		Handle: func(prev, cur interface{}) error {
			fmt.Printf("previous config: %+v -> ", prev)
			fmt.Printf("current config: %+v\n", cur)
			return nil
		},
	}
	custom  = flag.Int("custom", 0, "User application custom config")
	seconds = flag.Int("seconds", 10, "Seconds to exit this example")
)

func main() {
	_ = custom
	_, err := kconfig.New(Proxy, hdl)
	if err != nil {
		log.Fatalf("failed to create new config manager: %v", err)
	}
	fmt.Printf("Original config: %v (%v, %v)\n",
		Proxy.Get().(ExampleConfig).IntVal, Proxy.Value().IntVal, IntVal())

	time.Sleep(time.Second * time.Duration(*seconds))
}
