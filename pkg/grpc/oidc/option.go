package oidc

import "time"

type (
	Config struct {
		Timeout time.Duration
	}
	Option func(*Config)
)

func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}
