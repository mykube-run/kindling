package kconfig

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	clientv3 "go.etcd.io/etcd/client/v3"
	"os"
	"testing"
	"time"
)

type testConfig struct {
	IntVal      int            `mapstructure:"int"`
	StrVal      string         `mapstructure:"str"`
	ArrVal      []string       `mapstructure:"arr"`
	MapVal      map[string]int `mapstructure:"map"`
	EmbedStruct struct {
		IntVal int `mapstructure:"int"`
	} `mapstructure:"embed"`
	Child childConfig `mapstructure:"child"`
}

type childConfig struct {
	IntVal int    `mapstructure:"int"`
	StrVal string `mapstructure:"str"`
}

const (
	conf1 = `{"int": 42, "str": "foo", "arr": ["bar", "zee"], "map": {"foo": 42}, "embed": {"int": 42}, "child": {"int": 42, "str": "foo"}}`
	conf2 = `{"int": 36, "str": "another string", "arr": "foo", "map": {"bar": 36}, "embed": {"int": 36}, "child": {"int": 36, "str": "bar"}}`
)

func TestFileManager(t *testing.T) {
	var (
		c       = new(testConfig)
		fn      = "/tmp/kconfig-test.json"
		intVal  = 0
		newConf = func() interface{} {
			return new(testConfig)
		}
		handler = UpdateHandler{
			Name: "test",
			Handle: func(old, new interface{}) error {
				if v, ok := new.(*testConfig); !ok {
					return fmt.Errorf("invalid config type")
				} else {
					intVal = v.IntVal
					return nil
				}
			},
		}
	)
	log.Logger = log.Logger.Level(zerolog.TraceLevel)

	// Prepare the test config file
	_ = os.Remove(fn)
	if err := os.WriteFile(fn, []byte(conf1), os.ModePerm); err != nil {
		t.Fatalf("error writing to the test config file: %v", err)
	}

	// Test creating a new Manager
	opt := NewBootstrapOption().WithType(File).WithKey(fn)
	_, err := NewWithOption(c, newConf, opt, handler)
	if err != nil {
		t.Fatalf("error initializing Manager: %v", err)
	}
	checkConf1(c, t)

	// Change the config
	time.Sleep(time.Second * 5)
	if err = os.WriteFile(fn, []byte(conf2), os.ModePerm); err != nil {
		t.Fatalf("error writing new config to the test config file: %v", err)
	}
	time.Sleep(time.Second)
	checkConf2(c, t)

	if intVal != 36 {
		t.Fatalf("outer int val should be changed")
	}
}

func TestConsulManager(t *testing.T) {
	var (
		c       = new(testConfig)
		key     = "kconfig-test"
		addr    = "localhost:8500"
		intVal  = 0
		newConf = func() interface{} {
			return new(testConfig)
		}
		handler = UpdateHandler{
			Name: "test",
			Handle: func(old, new interface{}) error {
				if v, ok := new.(*testConfig); !ok {
					return fmt.Errorf("invalid config type")
				} else {
					intVal = v.IntVal
					return nil
				}
			},
		}
	)
	log.Logger = log.Logger.Level(zerolog.TraceLevel)

	// Prepare the config
	client, err := api.NewClient(&api.Config{Address: addr})
	if err != nil {
		t.Fatalf("failed to create consul client: %v", err)
	}
	_, _ = client.KV().Delete(key, nil)
	pair := api.KVPair{Key: key, Value: []byte(conf1)}
	_, err = client.KV().Put(&pair, nil)
	if err != nil {
		t.Fatalf("failed to write original config: %v", err)
	}

	// Test creating a new Manager
	opt := NewBootstrapOption().WithType(Consul).WithAddr(addr).WithKey(key)
	_, err = NewWithOption(c, newConf, opt, handler)
	if err != nil {
		t.Fatalf("error initializing Manager: %v", err)
	}
	checkConf1(c, t)

	// Change the config
	time.Sleep(time.Second * 5)
	pair = api.KVPair{Key: key, Value: []byte(conf2)}
	_, err = client.KV().Put(&pair, nil)
	if err != nil {
		t.Fatalf("failed to update new config: %v", err)
	}
	time.Sleep(time.Second)
	checkConf2(c, t)

	if intVal != 36 {
		t.Fatalf("outer int val should be changed")
	}
}

func TestEtcdManager(t *testing.T) {
	var (
		c       = new(testConfig)
		key     = "kconfig-test"
		addr    = "http://localhost:2379"
		intVal  = 0
		newConf = func() interface{} {
			return new(testConfig)
		}
		handler = UpdateHandler{
			Name: "test",
			Handle: func(old, new interface{}) error {
				if v, ok := new.(*testConfig); !ok {
					return fmt.Errorf("invalid config type")
				} else {
					intVal = v.IntVal
					return nil
				}
			},
		}
	)
	log.Logger = log.Logger.Level(zerolog.TraceLevel)

	// Prepare the config
	client, err := clientv3.New(clientv3.Config{Endpoints: []string{addr}})
	if err != nil {
		t.Fatalf("faild to create etcd client: %v", err)
	}
	ctx := context.TODO()
	_, _ = client.KV.Delete(ctx, key)
	_, err = client.KV.Put(ctx, key, conf1)
	if err != nil {
		t.Fatalf("failed to write original config: %v", err)
	}

	// Test creating a new Manager
	opt := NewBootstrapOption().WithType(Etcd).WithAddr(addr).WithKey(key)
	_, err = NewWithOption(c, newConf, opt, handler)
	if err != nil {
		t.Fatalf("error initializing Manager: %v", err)
	}
	checkConf1(c, t)

	// Change the config
	time.Sleep(time.Second * 5)
	_, err = client.KV.Put(ctx, key, conf2)
	if err != nil {
		t.Fatalf("failed to write new config: %v", err)
	}
	time.Sleep(time.Second)
	checkConf2(c, t)

	if intVal != 36 {
		t.Fatalf("outer int val should be changed")
	}
}

func checkConf1(conf *testConfig, t *testing.T) {
	if conf.IntVal != 42 {
		t.Fatalf("invalid config before update, int val: %v", conf.IntVal)
	}
	if conf.StrVal != "foo" {
		t.Fatalf("invalid config before update, str val: %v", conf.StrVal)
	}
	if len(conf.ArrVal) != 2 || conf.ArrVal[0] != "bar" || conf.ArrVal[1] != "zee" {
		t.Fatalf("invalid config before update, arr val: %v", conf.ArrVal)
	}
	if conf.MapVal == nil || len(conf.MapVal) != 1 || conf.MapVal["foo"] != 42 {
		t.Fatalf("invalid config before update, map val: %v", conf.MapVal)
	}
	if conf.EmbedStruct.IntVal != 42 {
		t.Fatalf("invalid config before update, embed struct val: %v", conf.EmbedStruct)
	}
	if conf.Child.IntVal != 42 || conf.Child.StrVal != "foo" {
		t.Fatalf("invalid config before update, child val: %v", conf.Child)
	}
}

func checkConf2(conf *testConfig, t *testing.T) {
	if conf.IntVal != 36 {
		t.Fatalf("invalid config after update, int val: %v", conf.IntVal)
	}
	if conf.StrVal != "another string" {
		t.Fatalf("invalid config after update, str val: %v", conf.StrVal)
	}
	if len(conf.ArrVal) != 1 || conf.ArrVal[0] != "foo" {
		t.Fatalf("invalid config after update, arr val: %v", conf.ArrVal)
	}
	if conf.MapVal == nil || len(conf.MapVal) != 1 || conf.MapVal["bar"] != 36 {
		t.Fatalf("invalid config after update, map val: %v", conf.MapVal)
	}
	if conf.EmbedStruct.IntVal != 36 {
		t.Fatalf("invalid config after update, embed struct val: %v", conf.EmbedStruct)
	}
	if conf.Child.IntVal != 36 || conf.Child.StrVal != "bar" {
		t.Fatalf("invalid config after update, child val: %v", conf.Child)
	}
}

func TestNewBootstrapOptionFromEnvFlag(t *testing.T) {
	opt := NewBootstrapOptionFromEnvFlag()
	if opt.Type != "" {
		t.Fatalf("type should be empty")
	}
}

func TestNewBootstrapOptionFromEnvFlag2(t *testing.T) {
	_ = os.Setenv("CONF_SRC_TYPE", "file")
	opt := NewBootstrapOptionFromEnvFlag()
	if opt.Type != "file" {
		t.Fatalf("type should be set to 'file'")
	}
}