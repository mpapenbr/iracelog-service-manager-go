package tenant

import (
	"context"
	"errors"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/tenant/v1/tenantv1connect"
	tenantv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/tenant/v1"
	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/permission"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/tenant"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/utils/cache"
)

func NewServer(opts ...Option) *tenantServer {
	ret := &tenantServer{
		log: log.Default().Named("grpc.tenant"),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Option func(*tenantServer)

func WithPool(p *pgxpool.Pool) Option {
	return func(srv *tenantServer) {
		srv.pool = p
	}
}

func WithPermissionEvaluator(pe permission.PermissionEvaluator) Option {
	return func(srv *tenantServer) {
		srv.pe = pe
	}
}

func WithTenantCache(arg cache.Cache[string, model.Tenant]) Option {
	return func(srv *tenantServer) {
		srv.cache = arg
	}
}

var ErrTenantNotFound = errors.New("tenant not found")

type tenantServer struct {
	x.UnimplementedTenantServiceHandler

	pe    permission.PermissionEvaluator
	pool  *pgxpool.Pool
	log   *log.Logger
	cache cache.Cache[string, model.Tenant]
}

//nolint:whitespace // can't make both editor and linter happy
func (s *tenantServer) GetTenants(
	ctx context.Context,
	req *connect.Request[tenantv1.GetTenantsRequest],
) (*connect.Response[tenantv1.GetTenantsResponse], error) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasPermission(a, permission.PermissionReadTenant) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	data, err := tenant.LoadAll(context.Background(), s.pool)
	if err != nil {
		return nil, err
	}
	ret := make([]*tenantv1.Tenant, len(data))
	for i, d := range data {
		ret[i] = d.Tenant
	}
	return connect.NewResponse(&tenantv1.GetTenantsResponse{Tenants: ret}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *tenantServer) GetTenant(
	ctx context.Context,
	req *connect.Request[tenantv1.GetTenantRequest],
) (*connect.Response[tenantv1.GetTenantResponse], error) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasPermission(a, permission.PermissionReadTenant) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	data, err := tenant.LoadBySelector(context.Background(), s.pool, req.Msg.Tenant)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&tenantv1.GetTenantResponse{Tenant: data.Tenant}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *tenantServer) CreateTenant(
	ctx context.Context, req *connect.Request[tenantv1.CreateTenantRequest],
) (*connect.Response[tenantv1.CreateTenantResponse], error) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasPermission(a, permission.PermissionCreateTenant) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	s.log.Debug("CreateTenant called",
		log.Any("arg", req.Msg),
	)

	var err error
	var ret *model.Tenant
	req.Msg.ApiKey = utils.HashAPIKey(req.Msg.ApiKey)
	ret, err = tenant.Create(ctx, s.pool, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&tenantv1.CreateTenantResponse{
		Tenant: ret.Tenant,
	}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *tenantServer) UpdateTenant(
	ctx context.Context, req *connect.Request[tenantv1.UpdateTenantRequest],
) (*connect.Response[tenantv1.UpdateTenantResponse], error) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasPermission(a, permission.PermissionUpdateTenant) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	s.log.Debug("UpdateTenant called",
		log.Any("arg", req.Msg),
	)
	var err error
	var ret *model.Tenant
	var t *model.Tenant
	t, err = tenant.LoadBySelector(ctx, s.pool, req.Msg.Tenant)
	if err != nil {
		return nil, err
	}
	if req.Msg.ApiKey != "" {
		req.Msg.ApiKey = utils.HashAPIKey(req.Msg.ApiKey)
	}
	ret, err = tenant.Update(ctx, s.pool, t.ID, req.Msg)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		s.cache.Invalidate(ctx, ret.APIKey)
	}
	return connect.NewResponse(&tenantv1.UpdateTenantResponse{
		Tenant: ret.Tenant,
	}), nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *tenantServer) DeleteTenant(
	ctx context.Context, req *connect.Request[tenantv1.DeleteTenantRequest],
) (*connect.Response[tenantv1.DeleteTenantResponse], error) {
	a := auth.FromContext(&ctx)
	if !s.pe.HasPermission(a, permission.PermissionDeleteTenant) {
		return nil, connect.NewError(connect.CodePermissionDenied, auth.ErrPermissionDenied)
	}
	s.log.Debug("DeleteTenant called",
		log.Any("arg", req.Msg))

	var err error
	data, err := tenant.LoadBySelector(context.Background(), s.pool, req.Msg.Tenant)
	if err != nil {
		return nil, err
	}
	var deleted int
	deleted, err = tenant.DeleteByID(ctx, s.pool, data.ID)
	if err != nil {
		return nil, err
	}
	if deleted == 0 {
		return nil, connect.NewError(connect.CodeNotFound, ErrTenantNotFound)
	}
	if s.cache != nil {
		s.cache.Invalidate(ctx, data.APIKey)
	}
	return connect.NewResponse(&tenantv1.DeleteTenantResponse{}), nil
}
