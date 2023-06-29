## konfig

### About

Rather than another config-parsing library, instead `konfig` aims to:

- load configurations from one centralized config provider
- **hot reload & unmarshalling** on configuration change
- encourage users to access configs via a single strong typing config instance

Supported config format (as for now):

- json
- yaml

Supported config source:

- File
- Consul
- Etcd
- Nacos

### Getting started

> Example code can be found at `cmd/konfig-example`.

#### 1. Implement `konfig.ConfigProxy`

> **NOTE**
> 1. The definition is pretty static, just copy and paste.

`config/main.go`

```go
package config

import (
     "github.com/mykube-run/kindling/pkg/konfig"
)

// Proxy konfig proxy
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

func (p *proxy) New() konfig.ConfigProxy {
     return &proxy{c: new(Sample)}
}

```

#### 2. Define config struct `Sample`

> **NOTE**
> 1. Accessing config blocks via an exported function is recommended (also the best practice), e.g. `config.DB()`
> 2. Always access current config from `Proxy` instance, which has the latest version of config,
     e.g. `Proxy.Get().(Sample).DB`
> 3. Field tags are declared via `mapstructure`.

`config/konfig.go`

```go
package config

// Sample is a config sample
type Sample struct {
     API         APIConfig         `mapstructure:"api"`
     DB          DatabaseConfig    `mapstructure:"db"`
     FeatureGate FeatureGateConfig `mapstructure:"feature_gate"`
}

// API returns the API config
func API() APIConfig {
     return Proxy.Get().(Sample).API
}

type APIConfig struct {
     ServiceXXXAddress string `mapstructure:"service_xxx_address"`
}

// DB returns the database config
func DB() DatabaseConfig {
     return Proxy.Get().(Sample).DB
}

type DatabaseConfig struct {
     Address  string `mapstructure:"address"`
     Username string `mapstructure:"username"`
     Password string `mapstructure:"password"`
}

// FeatureGate returns the feature gate config
func FeatureGate() FeatureGateConfig {
     return Proxy.Get().(Sample).FeatureGate
}

type FeatureGateConfig struct {
     EnableXXX bool `mapstructure:"enable_xxx"`
}

```

#### 3. Implement `konfig.ConfigUpdateHandler`

`handler.go`

```go
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
```

#### 4. Implement business code

`main.go`

```go
package main

import (
     "fmt"
     "github.com/mykube-run/kindling/cmd/konfig-example/config"
     "github.com/mykube-run/kindling/pkg/konfig"
     "log"
     "time"
)

func main() {
     _, err := konfig.New(config.Proxy, hdl1, hdl2)
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

```

#### 5. Create config

Start a config source, e.g. Consul, Etcd, Nacos (or create a config file).
Populate initial config with following value:

```json
{
  "api": {
    "service_xxx_address": "localhost:8000"
  },
  "db": {
    "address": "localhost:3306",
    "username": "test",
    "password": "test"
  },
  "feature_gate": {
    "enable_xxx": false
  }
}
```

#### 6. Start the demo

```bash
# Command line flags
go run ./cmd/konfig-example --conf-addr='localhost:8848' --conf-format=json \
  --conf-namespace=konfig --conf-group=test --conf-key=konfig-test --conf-type=nacos

# Env
CONF_ADDR='localhost:8500' CONF_FORMAT=json CONF_KEY=konfig-test CONF_TYPE=consul \
  go run ./cmd/konfig-example
```

Observe log output:

```text
* previous config: {API:{ServiceXXXAddress:} DB:{Address: Username: Password:} FeatureGate:{EnableXXX:false}}
* current config: {API:{ServiceXXXAddress:localhost:8000} DB:{Address:localhost:3306 Username:test Password:test} FeatureGate:{EnableXXX:false}}
* database address was set, connecting database...

{"level":"trace","module":"konfig","time":"2022-03-08T14:37:52+08:00","message":"handler [database] finished"}
{"level":"trace","module":"konfig","time":"2022-03-08T14:37:52+08:00","message":"handler [feature gate] finished"}
{"level":"info","module":"konfig","time":"2022-03-08T14:37:52+08:00","message":"updated config, md5: 900b5e06b577f1b8025e5502e048bdc6"}

* accessing database: localhost:3306...
* calling service xxx via address: localhost:8000...
```

#### 7. Update config and observe log output

```text
{"level":"trace","time":"2022-03-08T14:38:36+08:00","message":"namespace: konfig, group: test, key: konfig-test, md5: c8804a37eccf246fd87706c9eb1e7495"}
* previous config: {API:{ServiceXXXAddress:localhost:8000} DB:{Address:localhost:3306 Username:test Password:test} FeatureGate:{EnableXXX:false}}
* current config: {API:{ServiceXXXAddress:localhost:8080} DB:{Address:localhost:13306 Username:test Password:test} FeatureGate:{EnableXXX:true}}

* database address was updated, reconnecting database...
{"level":"trace","time":"2022-03-08T14:38:36+08:00","message":"handler [database] finished"}

* feature xxx was enabled, updating dependencies...
{"level":"trace","time":"2022-03-08T14:38:36+08:00","message":"handler [feature gate] finished"}
{"level":"info","time":"2022-03-08T14:38:36+08:00","message":"updated config, md5: c8804a37eccf246fd87706c9eb1e7495"}

* accessing database: localhost:13306...
* calling service xxx via address: localhost:8080...
* feature xxx was enabled, performing actions accordingly...
```