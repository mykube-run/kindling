package kconfig

import (
	"fmt"
	"github.com/mykube-run/kindling/pkg/kconfig/source"
	"github.com/mykube-run/kindling/pkg/types"
)

// ConfigSourceType specifies config sources that kconfig currently supports.
type ConfigSourceType string

const (
	File   ConfigSourceType = "file" // file can be json, yaml
	Etcd   ConfigSourceType = "etcd" // etcd v3
	Consul ConfigSourceType = "consul"
	Nacos  ConfigSourceType = "nacos"
)

func NewConfigSource(opt *BootstrapOption) (types.ConfigSource, error) {
	switch opt.Type {
	case File:
		return source.NewFileSource(opt.Key, opt.Logger)
	case Consul:
		return source.NewConsulSource(opt.Addrs[0], opt.Group, opt.Key, opt.Logger)
	case Etcd:
		return source.NewEtcdSource(opt.Addrs, opt.Group, opt.Key, opt.Logger)
	default:
		return nil, fmt.Errorf("unsupported config source type: %v", opt.Type)
	}
}
