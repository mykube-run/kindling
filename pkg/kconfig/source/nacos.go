package source

import (
	"fmt"
	"github.com/mykube-run/kindling/pkg/types"
	"github.com/mykube-run/kindling/pkg/utils"
	"github.com/nacos-group/nacos-sdk-go/clients"
	configclient "github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"net/url"
	"strconv"
	"strings"
)

var (
	NacosTimeout  uint64 = 5000
	NacosLogDir          = "/tmp/nacos/log"
	NacosCacheDir        = "/tmp/nacos/cache"
	NacosLogLevel        = "debug"
)

type Nacos struct {
	lg        types.Logger
	namespace string
	group     string
	key       string
	client    configclient.IConfigClient
	eventC    chan types.Event
	closing   bool
}

func NewNacosSource(addrs []string, namespace, group, key string, lg types.Logger) (types.ConfigSource, error) {
	cfg := constant.ClientConfig{
		NamespaceId:         namespace,
		TimeoutMs:           NacosTimeout,
		NotLoadCacheAtStart: true,
		LogDir:              NacosLogDir,
		CacheDir:            NacosCacheDir,
		LogLevel:            NacosLogLevel,
	}
	scs, err := ParseNacosAddrs(addrs)
	if err != nil {
		return nil, err
	}
	client, err := clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  &cfg,
		ServerConfigs: scs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create nacos config client: %w", err)
	}
	s := &Nacos{
		lg:        lg,
		namespace: namespace,
		group:     group,
		key:       key,
		client:    client,
		eventC:    make(chan types.Event, 1),
		closing:   false,
	}
	return s, nil
}

func (s *Nacos) Read() ([]byte, error) {
	v, err := s.client.GetConfig(vo.ConfigParam{
		DataId: s.key,
		Group:  s.group,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	return []byte(v), nil
}

func (s *Nacos) Watch() (<-chan types.Event, error) {
	fn := func(namespace, group, dataId, data string) {
		if s.closing {
			s.lg.Trace("nacos watcher has been closed, ignore event")
			return
		}

		byt := []byte(data)
		e := types.Event{
			Md5:  utils.Md5(byt),
			Data: byt,
		}
		s.lg.Trace(fmt.Sprintf("namespace: %v, group: %v, key: %v, md5: %v", s.namespace, s.group, s.key, e.Md5))
		s.eventC <- e
	}
	err := s.client.ListenConfig(vo.ConfigParam{
		DataId:   s.key,
		Group:    s.group,
		OnChange: fn,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to watch nacos config: %w", err)
	}
	return s.eventC, nil
}

func (s *Nacos) Close() error {
	s.closing = true
	if err := s.client.CancelListenConfig(vo.ConfigParam{
		DataId: s.key,
		Group:  s.group,
	}); err != nil {
		return err
	}
	close(s.eventC)
	return nil
}

func ParseNacosAddrs(addrs []string) ([]constant.ServerConfig, error) {
	scs := make([]constant.ServerConfig, 0, len(addrs))
	for _, v := range addrs {
		sc := constant.ServerConfig{}
		if !strings.HasPrefix(v, "http") {
			v = "http://" + v
		}
		u, err := url.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse url (%v): %w", v, err)
		}
		sc.Scheme = u.Scheme
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("failed to parse port (%v): %w", v, err)
		}
		sc.Port = uint64(port)
		spl := strings.Split(u.Host, ":")
		sc.IpAddr = spl[0]
		sc.ContextPath = u.Path
		scs = append(scs, sc)
	}
	return scs, nil
}
