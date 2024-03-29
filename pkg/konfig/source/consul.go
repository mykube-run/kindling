package source

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/mykube-run/kindling/pkg/log"
	"github.com/mykube-run/kindling/pkg/utils"
	"net"
	"time"
)

type consul struct {
	lg        log.Logger
	key       string
	client    *api.Client
	eventC    chan Event
	closing   bool
	lastIndex uint64
}

func NewConsulSource(addr string, group, key string, lg log.Logger) (ConfigSource, error) {
	cfg := api.DefaultConfig()
	cfg.Address = addr
	cfg.Transport.DialContext = (&net.Dialer{
		Timeout:   60 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	cfg.Transport.ResponseHeaderTimeout = 90 * time.Second
	cfg.Transport.ForceAttemptHTTP2 = false
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	s := &consul{
		lg:     lg,
		key:    genKey(group, key),
		eventC: make(chan Event, 1),
		client: client,
	}
	return s, nil
}

func (s *consul) Read() ([]byte, error) {
	pair, meta, err := s.client.KV().Get(s.key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	if pair == nil {
		return nil, fmt.Errorf("config key (%v) does not exist", s.key)
	}
	if meta != nil {
		s.lastIndex = meta.LastIndex
	}
	return pair.Value, nil
}

func (s *consul) Watch() (<-chan Event, error) {
	go s.watch()
	return s.eventC, nil
}

func (s *consul) Close() error {
	s.closing = true
	close(s.eventC)
	return nil
}

func (s *consul) watch() {
	for {
		if s.closing {
			s.lg.Trace("consul watcher has been closed, stop watching")
			return
		}
		// Blocks for at most 5s
		pair, meta, err := s.client.KV().Get(s.key, &api.QueryOptions{
			WaitIndex: s.lastIndex,
			WaitTime:  time.Second * 5,
		})
		if err != nil {
			s.lg.Error(fmt.Sprintf("error watching config: %v", err))
			continue
		}
		if pair == nil || meta == nil || meta.LastIndex <= s.lastIndex {
			continue
		}
		s.lastIndex = meta.LastIndex

		e := Event{
			Md5:  utils.Md5(pair.Value),
			Data: pair.Value,
		}
		s.lg.Trace(fmt.Sprintf("key: %v, new index: %v, md5: %v", s.key, s.lastIndex, e.Md5))
		if s.closing {
			s.lg.Trace("config source is closing, ignore event")
			return
		}
		s.eventC <- e
	}
}

func genKey(group, key string) string {
	if group != "" {
		key = group + "/" + key
	}
	return key
}
