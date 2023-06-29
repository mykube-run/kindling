package source

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/mykube-run/kindling/pkg/log"
	"github.com/mykube-run/kindling/pkg/utils"
	"io"
	"os"
)

type file struct {
	key     string
	watcher *fsnotify.Watcher
	eventC  chan Event
	closing bool
	lg      log.Logger
}

func NewFileSource(key string, lg log.Logger) (ConfigSource, error) {
	_, err := os.Stat(key)
	if err != nil {
		return nil, fmt.Errorf("invalid config file %v: %w", key, err)
	}
	s := &file{
		key:    key,
		eventC: make(chan Event, 1),
		lg:     lg,
	}
	return s, nil
}

func (s *file) Read() ([]byte, error) {
	byt, err := s.read()
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %v: %w", s.key, err)
	}
	return byt, nil
}

func (s *file) Watch() (<-chan Event, error) {
	if w, err := fsnotify.NewWatcher(); err != nil {
		return nil, fmt.Errorf("failed to initialize watcher: %w", err)
	} else {
		s.watcher = w
	}

	go func() {
		for {
			select {
			case evt, ok := <-s.watcher.Events:
				if !ok {
					s.lg.Trace("file watcher has been closed, stop watching")
					return
				}

				if evt.Op&fsnotify.Write == fsnotify.Write {
					s.handleEvent(evt)
				}
			case err, ok := <-s.watcher.Errors:
				if !ok {
					s.lg.Trace("file watcher has been closed, stop watching")
					return
				}
				s.lg.Error(fmt.Sprintf("file wacher error: %v", err))
			}
		}
	}()
	return s.eventC, s.watcher.Add(s.key)
}

func (s *file) Close() error {
	s.closing = true
	if s.watcher != nil {
		return s.watcher.Close()
	}
	close(s.eventC)
	return nil
}

func (s *file) read() ([]byte, error) {
	fn, err := os.OpenFile(s.key, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fn.Close() }()

	byt, err := io.ReadAll(fn)
	if err != nil {
		return nil, err
	}
	return byt, nil
}

func (s *file) handleEvent(evt fsnotify.Event) {
	byt, err := s.read()
	if err != nil {
		s.lg.Error(fmt.Sprintf("failed to read updated config: %v", err))
		return
	}
	e := Event{
		Md5:  utils.Md5(byt),
		Data: byt,
	}
	s.lg.Trace(fmt.Sprintf("file: %v, md5: %v", evt.Name, e.Md5))
	if s.closing {
		s.lg.Trace("config source is closing, ignore event")
		return
	}
	s.eventC <- e
}
