package cache

import (
	"context"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	utilsCache "github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache/loadercache"
)

// The key is the hashed api key
func NewTenantCache(r api.TenantRepository) utilsCache.Cache[string, model.Tenant] {
	return loadercache.New(loadercache.WithLoader(
		func(key string) (*model.Tenant, error) {
			return r.LoadByAPIKey(context.Background(), key)
		}))
}
