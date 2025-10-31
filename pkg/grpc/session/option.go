package session

import "time"

type (
	Config struct {
		Timeout    time.Duration
		CookieName string
	}
	Option func(*Config)
)

func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

func WithCookieName(name string) Option {
	return func(c *Config) {
		c.CookieName = name
	}
}
