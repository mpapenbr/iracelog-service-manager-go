package auth

import (
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache"
)

type (
	Config struct {
		AdminToken  string
		TenantCache cache.Cache[string, model.Tenant]
	}
	Option func(*Config)
)

func WithAdminToken(token string) Option {
	return func(c *Config) {
		c.AdminToken = token
	}
}

func WithTenantCache(arg cache.Cache[string, model.Tenant]) Option {
	return func(c *Config) {
		c.TenantCache = arg
	}
}
