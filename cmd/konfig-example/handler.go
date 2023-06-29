package main

import (
	"fmt"
	"github.com/mykube-run/kindling/cmd/konfig-example/config"
	"github.com/mykube-run/kindling/pkg/konfig"
)

// db demonstrates how database is reconnected,
// it would be a DB SINGLETON POINTER in real code
var db string

var hdl1 = konfig.ConfigUpdateHandler{
	Name: "database",
	Handle: func(prev, cur interface{}) error {
		fmt.Printf("* previous config: %+v\n", prev)
		fmt.Printf("* current config: %+v\n", cur)
		pc, _ := prev.(config.Sample)
		cc, _ := cur.(config.Sample)

		if cc.DB.Address != pc.DB.Address {
			if pc.DB.Address == "" {
				fmt.Printf("* database address was set, connecting database...\n")
				db = cc.DB.Address
				return nil
			}
			fmt.Printf("* database address was updated, reconnecting database...\n")
			db = cc.DB.Address
		}
		return nil
	},
}

var hdl2 = konfig.ConfigUpdateHandler{
	Name: "feature gate",
	Handle: func(prev, cur interface{}) error {
		pc, _ := prev.(config.Sample)
		cc, _ := cur.(config.Sample)

		if cc.FeatureGate.EnableXXX != pc.FeatureGate.EnableXXX &&
			cc.FeatureGate.EnableXXX {
			fmt.Printf("* feature xxx was enabled, updating dependencies...\n")
		}
		return nil
	},
}
