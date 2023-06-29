package retry

import (
	"github.com/rs/zerolog/log"
	"time"
)

// Simple retries fn for at most max times
func Simple(fn func() error, max int) error {
	mr := max
	err := fn()

	for err != nil && mr >= 0 {
		mr -= 1
		log.Debug().Err(err).Msg("retry func on error")
		err = fn()
	}
	return err
}

// WithInterval retries fn for at most max times, sleeps interval between each retry
func WithInterval(fn func() error, max int, interval time.Duration) error {
	mr := max
	err := fn()

	for err != nil && mr >= 0 {
		mr -= 1
		log.Debug().Err(err).Msg("retry func on error")
		time.Sleep(interval)
		err = fn()
	}
	return err
}
