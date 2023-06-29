package konfig

import (
	"fmt"
	"github.com/mykube-run/kindling/pkg/konfig/source"
)

func NewConfigSource(opt *BootstrapOption) (source.ConfigSource, error) {
	switch opt.Type {
	case source.File:
		return source.NewFileSource(opt.Key, opt.Logger)
	case source.Consul:
		return source.NewConsulSource(opt.Addrs[0], opt.Group, opt.Key, opt.Logger)
	case source.Etcd:
		return source.NewEtcdSource(opt.Addrs, opt.Group, opt.Key, opt.Logger)
	case source.Nacos:
		return source.NewNacosSource(opt.Addrs, opt.Namespace, opt.Group, opt.Key, opt.Logger)
	default:
		return nil, fmt.Errorf("unsupported config source type: %v", opt.Type)
	}
}
