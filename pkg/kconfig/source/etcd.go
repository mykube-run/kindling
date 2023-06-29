package source

import (
	"context"
	"fmt"
	"github.com/mykube-run/kindling/pkg/log"
	"github.com/mykube-run/kindling/pkg/utils"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

type etcd struct {
	key     string
	eventC  chan Event
	closing bool
	lg      log.Logger
	client  *clientv3.Client
}

func NewEtcdSource(addrs []string, group, key string, lg log.Logger) (ConfigSource, error) {
	cfg := clientv3.Config{
		Endpoints:            addrs,
		AutoSyncInterval:     time.Minute,
		DialTimeout:          time.Second * 2,
		DialKeepAliveTime:    time.Second * 5,
		DialKeepAliveTimeout: time.Second * 2,
		Username:             "",
		Password:             "",
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}
	s := &etcd{
		key:    genKey(group, key),
		eventC: make(chan Event, 1),
		lg:     lg,
		client: client,
	}
	return s, nil
}

func (s *etcd) Read() ([]byte, error) {
	ctx := context.Background()
	resp, err := s.client.KV.Get(ctx, s.key)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("config key does not exist")
	}
	return resp.Kvs[0].Value, nil
}

func (s *etcd) Watch() (<-chan Event, error) {
	c := s.client.Watch(context.Background(), s.key)
	go func() {
		for {
			select {
			case resp, ok := <-c:
				if !ok {
					s.lg.Trace("etcd watcher has been closed, stop watching")
					return
				}
				if resp.Err() != nil {
					s.lg.Error(fmt.Sprintf("etcd watch error: %v, stop watching", resp.Err()))
					return
				}
				for _, v := range resp.Events {
					if v.Type == clientv3.EventTypeDelete {
						continue
					}

					e := Event{
						Md5:  utils.Md5(v.Kv.Value),
						Data: v.Kv.Value,
					}
					s.lg.Trace(fmt.Sprintf("key: %v, new version: %v, md5: %v", s.key, v.Kv.Version, e.Md5))
					if s.closing {
						s.lg.Trace("config source is closing, ignore event")
						return
					}
					s.eventC <- e
				}
			}
		}
	}()
	return s.eventC, nil
}

func (s *etcd) Close() error {
	s.closing = true
	close(s.eventC)
	return nil
}
