package kconfig

import (
	"flag"
	"fmt"
	"github.com/mykube-run/kindling/pkg/types"
	"github.com/mykube-run/kindling/pkg/utils"
	"os"
	"strconv"
	"time"
)

// BootstrapOption is used to specify config source (and other additional) options.
type BootstrapOption struct {
	Type            types.ConfigSourceType
	Format          string
	Addrs           []string
	Namespace       string
	Group           string
	Key             string
	MinimalInterval time.Duration
	Logger          types.Logger
}

// NewBootstrapOption initializes a kconfig option
func NewBootstrapOption() *BootstrapOption {
	return &BootstrapOption{
		Format:          "json",
		MinimalInterval: time.Second * 5,
		Logger:          types.DefaultLogger,
	}
}

func NewBootstrapOptionFromEnvFlag() *BootstrapOption {
	opt := NewBootstrapOption()
	opt.parseEnvFlags()
	return opt
}

// WithType specifies config source type
func (opt *BootstrapOption) WithType(typ types.ConfigSourceType) *BootstrapOption {
	opt.Type = typ
	return opt
}

// WithAddr adds an address to the option
func (opt *BootstrapOption) WithAddr(addr string) *BootstrapOption {
	opt.Addrs = append(opt.Addrs, addr)
	return opt
}

// WithAddrs replaces option's addrs with given value
func (opt *BootstrapOption) WithAddrs(addrs []string) *BootstrapOption {
	opt.Addrs = addrs
	return opt
}

// WithIpPort adds an ip:port address to the option
func (opt *BootstrapOption) WithIpPort(ip, port interface{}) *BootstrapOption {
	opt.Addrs = append(opt.Addrs, fmt.Sprintf("%v:%v", ip, port))
	return opt
}

// WithNamespace specifies config namespace
func (opt *BootstrapOption) WithNamespace(ns string) *BootstrapOption {
	opt.Namespace = ns
	return opt
}

// WithGroup specifies config group
func (opt *BootstrapOption) WithGroup(group string) *BootstrapOption {
	opt.Group = group
	return opt
}

// WithKey specifies config key
func (opt *BootstrapOption) WithKey(key string) *BootstrapOption {
	opt.Key = key
	return opt
}

// WithMinimalInterval specifies a minimal duration that config can be updated, defaults to 5s.
// This prevents your application being destroyed by event storm.
func (opt *BootstrapOption) WithMinimalInterval(v time.Duration) *BootstrapOption {
	if v.Seconds() > 5 {
		opt.MinimalInterval = v
	}
	return opt
}

// WithLogger specifies a custom logger to the option
func (opt *BootstrapOption) WithLogger(lg types.Logger) *BootstrapOption {
	opt.Logger = lg
	return opt
}

// Validate checks option values
func (opt *BootstrapOption) Validate() error {
	if opt.Type == "" {
		return fmt.Errorf("config source type not provided")
	}
	switch opt.Type {
	case types.Consul, types.Etcd:
		if len(opt.Addrs) == 0 {
			return fmt.Errorf("config source address not provided")
		}
	}

	if opt.Key == "" {
		return fmt.Errorf("config key not provided")
	}
	if !(opt.Format == "json" || opt.Format == "yaml") {
		return fmt.Errorf("invalid config format: %v", opt.Format)
	}
	return nil
}

func (opt *BootstrapOption) parseEnvFlags() {
	var (
		typ       = flag.String("conf-src-type", "", "Config bootstrap option, config source type. Available options: file, etcd, consul, nacos.")
		format    = flag.String("conf-src-format", "", "Config bootstrap option, config format. Available options: json, yaml.")
		ip        = flag.String("conf-src-ip", "", "Config bootstrap option, config source ip, optional.")
		port      = flag.String("conf-src-port", "", "Config bootstrap option, config source port, only required when conf-ip is provided.")
		addr      = flag.String("conf-src-addr", "", "Config bootstrap option, config source address, multiple addresses can be given comma separated, e.g. 'ip1:2379,ip2:2379'.")
		namespace = flag.String("conf-src-namespace", "", "Config bootstrap option, config source namespace, optional.")
		group     = flag.String("conf-src-group", "", "Config bootstrap option, config source group, optional.")
		key       = flag.String("conf-src-key", "", "Config bootstrap option, config source key, required.")
		interval  = flag.String("conf-src-interval", "", "Config bootstrap option, minimal update interval in seconds, default to 5, optional.")
	)
	flag.Parse()

	otyp := types.ConfigSourceType(utils.If(*typ != "", *typ, os.Getenv("CONF_SRC_TYPE")).(string))
	oformat := utils.If(*format != "", *format, os.Getenv("CONF_SRC_FORMAT")).(string)
	oip := utils.If(*ip != "", *ip, os.Getenv("CONF_SRC_IP")).(string)
	oport := utils.If(*port != "", *port, os.Getenv("CONF_SRC_PORT")).(string)
	oaddr := utils.If(*addr != "", *addr, os.Getenv("CONF_SRC_ADDR")).(string)
	ons := utils.If(*namespace != "", *namespace, os.Getenv("CONF_SRC_NAMESPACE")).(string)
	ogroup := utils.If(*group != "", *group, os.Getenv("CONF_SRC_GROUP")).(string)
	okey := utils.If(*key != "", *key, os.Getenv("CONF_SRC_KEY")).(string)
	ointerval := utils.If(*interval != "", *interval, os.Getenv("CONF_SRC_INTERVAL")).(string)

	opt.Type = otyp
	opt.Namespace = ons
	opt.Group = ogroup
	opt.Key = okey
	iv, err := strconv.Atoi(ointerval)
	if err == nil && iv >= 5 {
		opt.MinimalInterval = time.Duration(iv)
	}
	addrs := utils.ParseCommaSeparated(oaddr)
	if len(addrs) == 0 {
		addrs = append(addrs, fmt.Sprintf("%v:%v", oip, oport))
	}
	opt.Addrs = addrs
	if oformat != "" {
		opt.Format = oformat
	}
}
