//nolint:whitespace // can't make both editor and linter happy
package tenant

import (
	"context"
	"fmt"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	tenantv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/tenant/v1"
	"github.com/aarondl/opt/omit"
	"github.com/gofrs/uuid/v5"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/db/models"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	api "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/api"
	bobCtx "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository/bob/context"
)

type (
	repo struct {
		conn bob.Executor
	}
)

var _ api.TenantRepository = (*repo)(nil)

func NewTenantRepository(conn bob.Executor) api.TenantRepository {
	return &repo{
		conn: conn,
	}
}

//nolint:whitespace // editor/linter issue
func (r *repo) Create(ctx context.Context, tenant *tenantv1.CreateTenantRequest) (
	*model.Tenant, error,
) {
	extID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	setter := &models.TenantSetter{
		Name:       omit.From(tenant.Name),
		APIKey:     omit.From(tenant.ApiKey),
		Active:     omit.From(tenant.IsActive),
		ExternalID: omit.From(extID),
	}
	ret, err := models.Tenants.Insert(setter).One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	return r.toModel(ret), nil
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadByID(ctx context.Context, id uint32) (
	*model.Tenant, error,
) {
	ret, err := models.Tenants.Query(
		models.SelectWhere.Tenants.ID.EQ(int32(id))).
		One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	return r.toModel(ret), nil
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadByExternalID(ctx context.Context, externalID string) (
	*model.Tenant, error,
) {
	ret, err := models.Tenants.Query(
		models.SelectWhere.Tenants.ExternalID.EQ(uuid.FromStringOrNil(externalID))).
		One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	return r.toModel(ret), nil
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadByAPIKey(ctx context.Context, apiKey string) (
	*model.Tenant, error,
) {
	ret, err := models.Tenants.Query(
		models.SelectWhere.Tenants.APIKey.EQ(apiKey)).
		One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	return r.toModel(ret), nil
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadByName(ctx context.Context, name string) (
	*model.Tenant, error,
) {
	ret, err := models.Tenants.Query(
		models.SelectWhere.Tenants.Name.EQ(name)).
		One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	return r.toModel(ret), nil
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadByEventID(ctx context.Context, eventID int) (
	*model.Tenant, error,
) {
	subQuery := psql.Select(
		sm.Columns(models.EventColumns.TenantID),
		sm.From(models.TableNames.Events),
		models.SelectWhere.Events.ID.EQ(int32(eventID)),
	)

	ret, err := models.Tenants.Query(
		sm.Where(models.TenantColumns.ID.EQ(subQuery)),
	).One(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	return r.toModel(ret), nil
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadBySelector(ctx context.Context, sel *commonv1.TenantSelector) (
	*model.Tenant, error,
) {
	switch sel.Arg.(type) {
	case *commonv1.TenantSelector_ExternalId:
		return r.LoadByExternalID(ctx, sel.GetExternalId().GetId())
	case *commonv1.TenantSelector_Name:
		return r.LoadByName(ctx, sel.GetName())
	default:
		return nil, fmt.Errorf("unknown selector %v", sel)
	}
}

//nolint:whitespace // editor/linter issue
func (r *repo) LoadAll(ctx context.Context) (
	[]*model.Tenant, error,
) {
	query := models.Tenants.Query(
		sm.OrderBy(models.TenantColumns.ID).Asc(),
	)
	data, err := query.All(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	ret := make([]*model.Tenant, 0)

	for i := range data {
		item := r.toModel(data[i])
		ret = append(ret, item)
	}
	return ret, nil
}

//nolint:whitespace //can't make both the linter and editor happy :(
func (r repo) Update(
	ctx context.Context,
	id uint32,
	req *tenantv1.UpdateTenantRequest) (
	*model.Tenant, error,
) {
	setter := &models.TenantSetter{
		Active: omit.From(req.IsActive),
	}
	if req.Name != "" {
		setter.Name = omit.From(req.Name)
	}
	if req.ApiKey != "" {
		setter.APIKey = omit.From(req.ApiKey)
	}
	_, err := models.Tenants.Update(
		setter.UpdateMod(),
		models.UpdateWhere.Tenants.ID.EQ(int32(id)),
	).Exec(ctx, r.getExecutor(ctx))
	if err != nil {
		return nil, err
	}
	return r.LoadByID(ctx, id)
}

// deletes an entry from the database, returns number of rows deleted.
func (r *repo) DeleteByID(ctx context.Context, id uint32) (int, error) {
	ret, err := models.Tenants.Delete(
		models.DeleteWhere.Tenants.ID.EQ(int32(id)),
	).Exec(ctx, r.getExecutor(ctx))
	return int(ret), err
}

func (r *repo) toModel(dbTenant *models.Tenant) *model.Tenant {
	var item model.Tenant
	item.ID = uint32(dbTenant.ID)
	item.APIKey = dbTenant.APIKey
	item.Tenant = &tenantv1.Tenant{
		ExternalId: &commonv1.UUID{Id: dbTenant.ExternalID.String()},
		Name:       dbTenant.Name,
		IsActive:   dbTenant.Active,
	}
	return &item
}

func (r *repo) getExecutor(ctx context.Context) bob.Executor {
	if executor := bobCtx.FromContext(ctx); executor != nil {
		return executor
	}
	return r.conn
}
