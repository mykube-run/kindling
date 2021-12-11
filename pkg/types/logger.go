package types

import "github.com/rs/zerolog/log"

type Logger interface {
	Trace(msg string)
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

var DefaultLogger = new(logger)

type logger struct {
}

func (lg *logger) Trace(msg string) {
	log.Trace().Str("module", "kconfig").Msg(msg)
}

func (lg *logger) Debug(msg string) {
	log.Debug().Str("module", "kconfig").Msg(msg)
}

func (lg *logger) Info(msg string) {
	log.Info().Str("module", "kconfig").Msg(msg)
}

func (lg *logger) Warn(msg string) {
	log.Warn().Str("module", "kconfig").Msg(msg)
}

func (lg *logger) Error(msg string) {
	log.Error().Str("module", "kconfig").Msg(msg)
}
