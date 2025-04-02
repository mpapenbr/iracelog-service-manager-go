package util

import (
	"context"
	"errors"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/config"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/tenant"
)

var ErrTenantSelectorRequired = errors.New("tenant selector is required")

// returns the tenant for the given selector if tenants are supported
// if tenants are supported a selector must be provided
//
//nolint:whitespace // can't make both editor and linter happy
func ResolveTenant(
	ctx context.Context,
	conn repository.Querier,
	sel *commonv1.TenantSelector,
) (*model.Tenant, error) {
	var data *model.Tenant
	var err error
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return nil, nil
	}
	// if tenants are not supported, always return nil
	if !cfg.SupportTenants {
		return nil, nil
	}

	if sel == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			ErrTenantSelectorRequired)
	}
	switch sel.Arg.(type) {
	case *commonv1.TenantSelector_ExternalId:
		data, err = tenant.LoadByExternalID(ctx, conn, sel.GetExternalId().GetId())
	case *commonv1.TenantSelector_Name:
		data, err = tenant.LoadByName(ctx, conn, sel.GetName())
	default:
		return nil, nil
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, repository.ErrNoData)
		}
		return nil, err
	}
	return data, nil
}
