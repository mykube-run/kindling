package log

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
	log.Trace().Msg(msg)
}

func (lg *logger) Debug(msg string) {
	log.Debug().Msg(msg)
}

func (lg *logger) Info(msg string) {
	log.Info().Msg(msg)
}

func (lg *logger) Warn(msg string) {
	log.Warn().Msg(msg)
}

func (lg *logger) Error(msg string) {
	log.Error().Msg(msg)
}
