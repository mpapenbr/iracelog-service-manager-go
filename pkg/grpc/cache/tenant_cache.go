package cache

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/tenant"
	utilsCache "github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache/loadercache"
)

// The key is the hashed api key
func NewTenantCache(pool *pgxpool.Pool) utilsCache.Cache[string, model.Tenant] {
	return loadercache.New(loadercache.WithLoader(
		func(key string) (*model.Tenant, error) {
			return tenant.LoadByApiKey(context.Background(), pool, key)
		}))
}
