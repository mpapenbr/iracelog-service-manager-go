//nolint:whitespace // can't make both editor and linter happy
package tenant

import (
	"context"
	"fmt"

	commonv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/common/v1"
	tenantv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/tenant/v1"
	"github.com/jackc/pgx/v5"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/repository"
)

var selector = `select t.id, t.external_id, t.name, t.api_key, t.active
	from tenant t`

func Create(
	ctx context.Context,
	conn repository.Querier,
	tenant *tenantv1.CreateTenantRequest,
) (*tenantv1.Tenant, error) {
	row := conn.QueryRow(ctx, `
	insert into tenant (
		name, api_key, active, external_id
	) values ($1,$2,$3,uuid_generate_v4())
	returning id
		`,
		tenant.Name, tenant.ApiKey, tenant.IsActive,
	)
	var tenantId uint32
	if err := row.Scan(&tenantId); err != nil {
		return nil, err
	}
	return LoadById(ctx, conn, tenantId)
}

func LoadById(ctx context.Context, conn repository.Querier, id uint32) (
	*tenantv1.Tenant, error,
) {
	row := conn.QueryRow(ctx, fmt.Sprintf("%s where t.id=$1", selector), id)

	return readData(row)
}

func LoadByExternalId(ctx context.Context, conn repository.Querier, externalId string) (
	*tenantv1.Tenant, error,
) {
	row := conn.QueryRow(ctx,
		fmt.Sprintf("%s where t.external_id=$1", selector), externalId)

	return readData(row)
}

func LoadByApiKey(ctx context.Context, conn repository.Querier, apiKey string) (
	*tenantv1.Tenant, error,
) {
	row := conn.QueryRow(ctx, fmt.Sprintf("%s where t.api_key=$1", selector), apiKey)

	return readData(row)
}

func LoadByName(ctx context.Context, conn repository.Querier, name string) (
	*tenantv1.Tenant, error,
) {
	row := conn.QueryRow(ctx, fmt.Sprintf("%s where t.name=$1", selector), name)

	return readData(row)
}

func LoadByEventId(ctx context.Context, conn repository.Querier, eventId int) (
	*tenantv1.Tenant, error,
) {
	row := conn.QueryRow(ctx,
		fmt.Sprintf("%s where t.id=(select tenant_id from event where id=$1)", selector),
		eventId)
	return readData(row)
}

func LoadAll(ctx context.Context, conn repository.Querier) (
	[]*tenantv1.Tenant, error,
) {
	row, err := conn.Query(ctx, fmt.Sprintf("%s order by t.id asc", selector))
	if err != nil {
		return nil, err
	}
	ret := make([]*tenantv1.Tenant, 0)
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

func Update(
	ctx context.Context,
	conn repository.Querier,
	tenant *tenantv1.UpdateTenantRequest,
) (*tenantv1.Tenant, error) {
	cmdTag, err := conn.Exec(ctx, `
		update tenant set name=$1, api_key=$2, active=$3 where id=$4
	`, tenant.Name, tenant.ApiKey, tenant.IsActive, tenant.Id)
	if err != nil {
		return nil, err
	}
	if cmdTag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	return LoadById(ctx, conn, tenant.Id)
}

// deletes an entry from the database, returns number of rows deleted.
func DeleteById(ctx context.Context, conn repository.Querier, id uint32) (int, error) {
	cmdTag, err := conn.Exec(ctx, "delete from tenant where id=$1", id)
	if err != nil {
		return 0, err
	}
	return int(cmdTag.RowsAffected()), nil
}

func readData(row pgx.Row) (*tenantv1.Tenant, error) {
	var item tenantv1.Tenant
	var extId string
	if err := row.Scan(
		&item.Id,
		&extId,
		&item.Name,
		&item.ApiKey,
		&item.IsActive,
	); err != nil {
		return nil, err
	}
	item.ExternalId = &commonv1.UUID{Id: extId}
	return &item, nil
}
