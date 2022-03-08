## kconfig

### About

Rather than another config-parsing library, instead `kconfig` aims to:

- load configurations from one centralized config provider
- hot reload on configuration change
- encourage users to access configs via a single strong typing config instance

### Getting started

#### 1. Implement `types.ConfigProxy`

> **NOTE**
> 1. The definition is pretty static, just copy and paste.

`config/main.go`

```go
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

func (p *proxy) New() types.ConfigProxy {
	return &proxy{c: new(Sample)}
}
```

#### 2. Define config struct `Sample`

> **NOTE**
> 1. Accessing config blocks via an exported function is recommended (also the best practice), e.g. `config.DB()`
> 2. Always access current config from `Proxy` instance, which has the latest version of config, e.g. `Proxy.Get().(Sample).DB`
> 3. Field tags are declared via `mapstructure`.

`config/types.go`

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

#### 3. Implement `types.ConfigUpdateHandler` 

`handler.go`

```go
// db demonstrates how database is reconnected,
// it would be a DB SINGLETON POINTER in real code
var db string

var hdl1 = types.ConfigUpdateHandler{
	Name: "database",
	Handle: func(prev, cur interface{}) error {
		fmt.Printf("* previous config: %+v\n", prev)
		fmt.Printf("* current config: %+v\n", cur)
		pc, _ := prev.(ConfigSample)
		cc, _ := cur.(ConfigSample)

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

var hdl2 = types.ConfigUpdateHandler{
	Name: "feature gate",
	Handle: func(prev, cur interface{}) error {
		pc, _ := prev.(ConfigSample)
		cc, _ := cur.(ConfigSample)

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
go run ./cmd/kconfig-example --conf-addr='localhost:8848' --conf-format=json \
  --conf-namespace=kconfig --conf-group=test --conf-key=kconfig-test --conf-type=nacos

# Env
CONF_ADDR='localhost:8500' CONF_FORMAT=json CONF_KEY=kconfig-test CONF_TYPE=consul \
  go run ./cmd/kconfig-example
```

Observe log output:

```text
* previous config: {API:{ServiceXXXAddress:} DB:{Address: Username: Password:} FeatureGate:{EnableXXX:false}}
* current config: {API:{ServiceXXXAddress:localhost:8000} DB:{Address:localhost:3306 Username:test Password:test} FeatureGate:{EnableXXX:false}}
* database address was set, connecting database...

{"level":"trace","module":"kconfig","time":"2022-03-08T14:37:52+08:00","message":"handler [database] finished"}
{"level":"trace","module":"kconfig","time":"2022-03-08T14:37:52+08:00","message":"handler [feature gate] finished"}
{"level":"info","module":"kconfig","time":"2022-03-08T14:37:52+08:00","message":"updated config, md5: 900b5e06b577f1b8025e5502e048bdc6"}

* accessing database: localhost:3306...
* calling service xxx via address: localhost:8000...
```

#### 7. Update config and observe log output

```text
{"level":"trace","module":"kconfig","time":"2022-03-08T14:38:36+08:00","message":"namespace: kconfig, group: test, key: kconfig-test, md5: c8804a37eccf246fd87706c9eb1e7495"}
* previous config: {API:{ServiceXXXAddress:localhost:8000} DB:{Address:localhost:3306 Username:test Password:test} FeatureGate:{EnableXXX:false}}
* current config: {API:{ServiceXXXAddress:localhost:8080} DB:{Address:localhost:13306 Username:test Password:test} FeatureGate:{EnableXXX:true}}

* database address was updated, reconnecting database...
{"level":"trace","module":"kconfig","time":"2022-03-08T14:38:36+08:00","message":"handler [database] finished"}

* feature xxx was enabled, updating dependencies...
{"level":"trace","module":"kconfig","time":"2022-03-08T14:38:36+08:00","message":"handler [feature gate] finished"}
{"level":"info","module":"kconfig","time":"2022-03-08T14:38:36+08:00","message":"updated config, md5: c8804a37eccf246fd87706c9eb1e7495"}

* accessing database: localhost:13306...
* calling service xxx via address: localhost:8080...
* feature xxx was enabled, performing actions accordingly...
```