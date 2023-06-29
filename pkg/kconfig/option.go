package kconfig

import (
	"flag"
	"fmt"
	"github.com/mykube-run/kindling/pkg/kconfig/source"
	"github.com/mykube-run/kindling/pkg/log"
	"github.com/mykube-run/kindling/pkg/utils"
	"os"
	"strconv"
	"time"
)

// BootstrapOption is used to specify config source (and other additional) options.
type BootstrapOption struct {
	Type            source.ConfigSourceType
	Format          string
	Addrs           []string
	Namespace       string
	Group           string
	Key             string
	MinimalInterval time.Duration
	Logger          log.Logger
}

// NewBootstrapOption initializes a bootstrap config option
func NewBootstrapOption() *BootstrapOption {
	return &BootstrapOption{
		Format:          "json",
		MinimalInterval: time.Second * 5,
		Logger:          log.DefaultLogger,
	}
}

// NewBootstrapOptionFromEnvFlag initializes a bootstrap config option from environments & flags.
// Flag value has higher priority when both given in environments & flags.
// NOTE:
//		1) Flags are parsed once this function is called.
// 		2) Customize this function if needed
var NewBootstrapOptionFromEnvFlag = func() *BootstrapOption {
	opt := NewBootstrapOption()
	opt.parseEnvFlags()
	return opt
}

// WithType specifies config source type
func (opt *BootstrapOption) WithType(typ source.ConfigSourceType) *BootstrapOption {
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
func (opt *BootstrapOption) WithLogger(lg log.Logger) *BootstrapOption {
	opt.Logger = lg
	return opt
}

// Validate checks option values
func (opt *BootstrapOption) Validate() error {
	if opt.Type == "" {
		return fmt.Errorf("config source type not provided")
	}
	switch opt.Type {
	case source.Consul, source.Etcd:
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

var (
	typ       = flag.String("conf-type", "", "Bootstrap config option, config source type. Available options: file, etcd, consul, nacos.")
	format    = flag.String("conf-format", "", "Bootstrap config option, config format. Available options: json, yaml.")
	ip        = flag.String("conf-ip", "", "Bootstrap config option, config source ip, optional.")
	port      = flag.String("conf-port", "", "Bootstrap config option, config source port, only required when conf-ip is provided.")
	addr      = flag.String("conf-addr", "", "Bootstrap config option, config source address, multiple addresses can be given comma separated, e.g. 'ip1:2379,ip2:2379'.")
	namespace = flag.String("conf-namespace", "", "Bootstrap config option, config namespace, optional.")
	group     = flag.String("conf-group", "", "Bootstrap config option, config group, optional.")
	key       = flag.String("conf-key", "", "Bootstrap config option, config key, required.")
	interval  = flag.String("conf-interval", "", "Bootstrap config option, minimal update interval in seconds, default to 5, optional.")
)

func (opt *BootstrapOption) parseEnvFlags() {
	if !flag.Parsed() {
		flag.Parse()
	}

	otyp := source.ConfigSourceType(utils.If(*typ != "", *typ, os.Getenv("CONF_TYPE")).(string))
	oformat := utils.If(*format != "", *format, os.Getenv("CONF_FORMAT")).(string)
	oip := utils.If(*ip != "", *ip, os.Getenv("CONF_IP")).(string)
	oport := utils.If(*port != "", *port, os.Getenv("CONF_PORT")).(string)
	oaddr := utils.If(*addr != "", *addr, os.Getenv("CONF_ADDR")).(string)
	ons := utils.If(*namespace != "", *namespace, os.Getenv("CONF_NAMESPACE")).(string)
	ogroup := utils.If(*group != "", *group, os.Getenv("CONF_GROUP")).(string)
	okey := utils.If(*key != "", *key, os.Getenv("CONF_KEY")).(string)
	ointerval := utils.If(*interval != "", *interval, os.Getenv("CONF_INTERVAL")).(string)

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
