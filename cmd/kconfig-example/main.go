package main

import (
	"fmt"
	"github.com/mykube-run/kindling/cmd/kconfig-example/config"
	"github.com/mykube-run/kindling/pkg/kconfig"
	"log"
	"time"
)

func main() {
	_, err := kconfig.New(config.Proxy, hdl1, hdl2)
	if err != nil {
		log.Fatalf("failed to create new config manager: %v", err)
	}

	for {
		time.Sleep(5 * time.Second)
		// Accessing db via singleton
		fmt.Printf("* accessing database: %v...\n", db)
		// Accessing config values directly
		fmt.Printf("* calling service xxx via address: %v...\n", config.API().ServiceXXXAddress)
		if config.FeatureGate().EnableXXX {
			fmt.Printf("* feature xxx was enabled, performing actions accordingly...\n")
		}
	}
}
