package cache

import (
	"context"

	tenantv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/tenant/v1"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/tenant"
	utilsCache "github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache/loadercache"
)

func NewTenantCache(pool *pgxpool.Pool) utilsCache.Cache[string, tenantv1.Tenant] {
	return loadercache.New(loadercache.WithLoader(
		func(key string) (*tenantv1.Tenant, error) {
			return tenant.LoadByApiKey(context.Background(), pool, key)
		}))
}
