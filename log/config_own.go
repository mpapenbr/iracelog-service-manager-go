package log

import (
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/otel"
)

type (
	// based on this config the logger is created
	//nolint:lll // readabilty
	loggerConfig struct {
		cfg                 *Config
		level               string          // optional, if empty, use default level from config
		telemetry           *otel.Telemetry // optional, if nil, no otel logging
		removeContextFields bool            // if true, remove context fields from the log
		useZap              bool            // if true, use configured zap
	}
	ConfigOption interface {
		apply(*loggerConfig) *loggerConfig
	}
	optFunc func(*loggerConfig) *loggerConfig
)

func (f optFunc) apply(c *loggerConfig) *loggerConfig { return f(c) }

func newLoggerConfig(opts ...ConfigOption) *loggerConfig {
	ret := &loggerConfig{
		cfg:                 DefaultProdConfig(),
		telemetry:           nil,
		removeContextFields: true,
		useZap:              true, // use output via zap by default
	}
	for _, opt := range opts {
		opt.apply(ret)
	}
	if ret.level == "" {
		ret.level = ret.cfg.DefaultLevel
	}
	return ret
}

func WithLogConfig(arg *Config) ConfigOption {
	return optFunc(func(c *loggerConfig) *loggerConfig {
		c.cfg = arg
		return c
	})
}

func WithLogLevel(arg string) ConfigOption {
	return optFunc(func(c *loggerConfig) *loggerConfig {
		c.level = arg
		return c
	})
}

func WithTelemetry(arg *otel.Telemetry) ConfigOption {
	return optFunc(func(c *loggerConfig) *loggerConfig {
		c.telemetry = arg
		return c
	})
}

// if context is put into fields this is often used for OTLP logging
// in log files these attrs are not useful, so we can remove them
func WithRemoveContextFields(arg bool) ConfigOption {
	return optFunc(func(c *loggerConfig) *loggerConfig {
		c.removeContextFields = arg
		return c
	})
}

// if true, use the configured zap logger for output
// if false no output will be sent to files or console
// deactivating zap may be useful if only otel logging should be used
func WithUseZap(arg bool) ConfigOption {
	return optFunc(func(c *loggerConfig) *loggerConfig {
		c.useZap = arg
		return c
	})
}
