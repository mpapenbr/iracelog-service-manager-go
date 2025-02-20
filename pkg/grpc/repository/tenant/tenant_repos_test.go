//nolint:dupl,funlen,errcheck,gocognit //ok for this test code
package tenant

import (
	"context"
	"log"
	"reflect"
	"testing"

	tenantv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/tenant/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	"github.com/mpapenbr/iracelog-service-manager-go/testsupport/testdb"
)

var sampleTrack = &tenantv1.CreateTenantRequest{
	Name:     "testtenant",
	ApiKey:   "testapikey",
	IsActive: true,
}

func createSampleEntry(db *pgxpool.Pool) *tenantv1.Tenant {
	ctx := context.Background()
	var ret *tenantv1.Tenant
	err := pgx.BeginFunc(context.Background(), db, func(tx pgx.Tx) error {
		var err error
		ret, err = Create(ctx, tx, sampleTrack)
		return err
	})
	if err != nil {
		log.Fatalf("createSampleEntry: %v\n", err)
	}
	return ret
}

func TestCreate(t *testing.T) {
	pool := testdb.InitTestDb()
	type args struct {
		req *tenantv1.CreateTenantRequest
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "new entry",
			args: args{req: &tenantv1.CreateTenantRequest{
				Name:     "newname",
				ApiKey:   "apikey",
				IsActive: true,
			}},
			wantErr: false,
		},
		{
			name: "duplicate name",
			args: args{&tenantv1.CreateTenantRequest{
				Name:     "testtenant",
				ApiKey:   "someapikey",
				IsActive: true,
			}},
			wantErr: true,
		},
		{
			name: "duplicate api_key",
			args: args{&tenantv1.CreateTenantRequest{
				Name:     "duplicateapikey",
				ApiKey:   "testapikey",
				IsActive: true,
			}},
			wantErr: true,
		},
	}
	createSampleEntry(pool)
	for _, tt := range tests {
		ctx := context.Background()
		t.Run(tt.name, func(t *testing.T) {
			_, err := Create(ctx, pool, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create error = %v, wantErr %v",
					err, tt.wantErr)
			}
		})
	}
}

func TestLoadById(t *testing.T) {
	pool := testdb.InitTestDb()
	sample := createSampleEntry(pool)
	type args struct {
		id uint32
	}
	tests := []struct {
		name    string
		args    args
		want    *tenantv1.Tenant
		wantErr bool
	}{
		{
			name: "existing entry",
			args: args{id: sample.Id},
			want: sample,
		},
		{
			name:    "unknown entry",
			args:    args{id: 999},
			wantErr: true,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				got, err := LoadById(ctx, c.Conn(), tt.args.id)
				if (err != nil) != tt.wantErr {
					t.Errorf("LoadEventById() error = %v, wantErr %v", err, tt.wantErr)
					return err
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("LoadEventById() = %v, want %v", got, tt.want)
				}
				return nil
			})
		})
	}
}

func TestDeleteById(t *testing.T) {
	db := testdb.InitTestDb()
	sample := createSampleEntry(db)

	type args struct {
		id uint32
	}
	tests := []struct {
		name string

		args    args
		want    int
		wantErr bool
	}{
		{
			name: "delete_existing",
			args: args{id: sample.Id},
			want: 1,
		},
		{
			name: "delete_non_existing",
			args: args{id: 0}, // doesn't exist
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				got, err := DeleteById(ctx, c.Conn(), tt.args.id)
				if (err != nil) != tt.wantErr {
					t.Errorf("DeleteById() error = %v, wantErr %v", err, tt.wantErr)
					return nil
				}
				if got != tt.want {
					t.Errorf("DeleteById() = %v, want %v", got, tt.want)
				}
				return nil
			})
		})
	}
}

func TestUpdate(t *testing.T) {
	db := testdb.InitTestDb()

	type args struct {
		apply func(req *tenantv1.UpdateTenantRequest)
	}
	tests := []struct {
		name    string
		args    args
		verify  func(t *testing.T, preUpdate, actual *tenantv1.Tenant) bool
		wantErr bool
	}{
		{
			name: "update name",
			args: args{apply: func(req *tenantv1.UpdateTenantRequest) {
				req.Name = "newname"
			}},
			verify: func(t *testing.T, preUpdate, actual *tenantv1.Tenant) bool {
				t.Helper()
				assert.Equal(t, "newname", actual.Name, "name updated")
				assert.Equal(t, preUpdate.ApiKey, actual.ApiKey, "apiKey unchanged")
				assert.Equal(t, preUpdate.IsActive, actual.IsActive, "isActive unchanged")
				return true
			},
		},
		{
			name: "update apiKey",
			args: args{apply: func(req *tenantv1.UpdateTenantRequest) {
				req.ApiKey = "newapikey"
			}},
			verify: func(t *testing.T, preUpdate, actual *tenantv1.Tenant) bool {
				t.Helper()
				assert.Equal(t, preUpdate.Name, actual.Name, "name unchanged")
				assert.Equal(t, "newapikey", actual.ApiKey, "apiKey changed")
				assert.Equal(t, preUpdate.IsActive, actual.IsActive, "isActive unchanged")
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				sample := createSampleEntry(db)
				defer DeleteById(ctx, c.Conn(), sample.Id)
				req := &tenantv1.UpdateTenantRequest{
					Id:       sample.Id,
					IsActive: sample.IsActive,
				}
				tt.args.apply(req)
				got, err := Update(ctx, c.Conn(), req)

				if (err != nil) != tt.wantErr {
					t.Errorf("UpdateById() error = %v, wantErr %v", err, tt.wantErr)
					return nil
				}
				if !tt.verify(t, sample, got) {
					t.Errorf("UpdateById()")
				}
				return nil
			})
		})
	}
}
