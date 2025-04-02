//nolint:whitespace // can't make both editor and linter happy
package tenant

import (
	"context"
	"errors"
	"fmt"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	tenantv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/tenant/v1"
	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/model"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
)

var selector = `select t.id, t.external_id, t.name, t.api_key, t.active
	from tenant t`

func Create(
	ctx context.Context,
	conn repository.Querier,
	tenant *tenantv1.CreateTenantRequest,
) (*model.Tenant, error) {
	row := conn.QueryRow(ctx, `
	insert into tenant (
		name, api_key, active, external_id
	) values ($1,$2,$3,uuid_generate_v4())
	returning id
		`,
		tenant.Name, tenant.ApiKey, tenant.IsActive,
	)
	var tenantID uint32
	if err := row.Scan(&tenantID); err != nil {
		return nil, err
	}
	return LoadByID(ctx, conn, tenantID)
}

func LoadByID(ctx context.Context, conn repository.Querier, id uint32) (
	*model.Tenant, error,
) {
	row := conn.QueryRow(ctx, fmt.Sprintf("%s where t.id=$1", selector), id)

	return readData(row)
}

func LoadByExternalID(ctx context.Context, conn repository.Querier, externalID string) (
	*model.Tenant, error,
) {
	row := conn.QueryRow(ctx,
		fmt.Sprintf("%s where t.external_id=$1", selector), externalID)

	return readData(row)
}

func LoadByAPIKey(ctx context.Context, conn repository.Querier, apiKey string) (
	*model.Tenant, error,
) {
	row := conn.QueryRow(ctx, fmt.Sprintf("%s where t.api_key=$1", selector), apiKey)

	return readData(row)
}

func LoadByName(ctx context.Context, conn repository.Querier, name string) (
	*model.Tenant, error,
) {
	row := conn.QueryRow(ctx, fmt.Sprintf("%s where t.name=$1", selector), name)

	return readData(row)
}

func LoadByEventID(ctx context.Context, conn repository.Querier, eventID int) (
	*model.Tenant, error,
) {
	row := conn.QueryRow(ctx,
		fmt.Sprintf("%s where t.id=(select tenant_id from event where id=$1)", selector),
		eventID)
	return readData(row)
}

func LoadAll(ctx context.Context, conn repository.Querier) (
	[]*model.Tenant, error,
) {
	row, err := conn.Query(ctx, fmt.Sprintf("%s order by t.id asc", selector))
	if err != nil {
		return nil, err
	}
	ret := make([]*model.Tenant, 0)
	defer row.Close()
	for row.Next() {
		item, err := readData(row)
		if err != nil {
			return nil, err
		}
		ret = append(ret, item)

	}
	return ret, nil
}

func LoadBySelector(
	ctx context.Context,
	conn repository.Querier,
	sel *commonv1.TenantSelector,
) (*model.Tenant, error) {
	var data *model.Tenant
	var err error

	switch sel.Arg.(type) {
	case *commonv1.TenantSelector_ExternalId:
		data, err = LoadByExternalID(ctx, conn, sel.GetExternalId().GetId())
	case *commonv1.TenantSelector_Name:
		data, err = LoadByName(ctx, conn, sel.GetName())
	default:
		return nil, fmt.Errorf("unknown selector %v", sel)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrNoData
		}
		return nil, err
	}
	return data, nil
}

func Update(
	ctx context.Context,
	conn repository.Querier,
	id uint32,
	tenant *tenantv1.UpdateTenantRequest,
) (*model.Tenant, error) {
	cmdTag, err := conn.Exec(ctx, `
		update tenant set
		name=coalesce(nullif($1,''),name),
		api_key=coalesce(nullif($2,''),api_key),
		active=$3
		where id=$4
	`, tenant.Name, tenant.ApiKey, tenant.IsActive, id)
	if err != nil {
		return nil, err
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	return LoadByID(ctx, conn, id)
}

// deletes an entry from the database, returns number of rows deleted.
func DeleteByID(ctx context.Context, conn repository.Querier, id uint32) (int, error) {
	cmdTag, err := conn.Exec(ctx, "delete from tenant where id=$1", id)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func readData(row pgx.Row) (*model.Tenant, error) {
	var item model.Tenant
	var t tenantv1.Tenant
	var extID string
	if err := row.Scan(
		&item.ID,
		&extID,
		&t.Name,
		&item.APIKey,
		&t.IsActive,
	); err != nil {
		return nil, err
	}
	t.ExternalId = &commonv1.UUID{Id: extID}
	item.Tenant = &t
	return &item, nil
}
