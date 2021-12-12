package kconfig

import (
	"fmt"
	"github.com/mykube-run/kindling/pkg/kconfig/source"
	"github.com/mykube-run/kindling/pkg/types"
)

func NewConfigSource(opt *BootstrapOption) (types.ConfigSource, error) {
	switch opt.Type {
	case types.File:
		return source.NewFileSource(opt.Key, opt.Logger)
	case types.Consul:
		return source.NewConsulSource(opt.Addrs[0], opt.Group, opt.Key, opt.Logger)
	case types.Etcd:
		return source.NewEtcdSource(opt.Addrs, opt.Group, opt.Key, opt.Logger)
	default:
		return nil, fmt.Errorf("unsupported config source type: %v", opt.Type)
	}
}
