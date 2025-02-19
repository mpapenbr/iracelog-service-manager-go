//nolint:dupl,funlen,errcheck,gocognit //ok for this test code
package permission

import (
	_ "embed"
	"testing"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
)

type TestAuth struct {
	auth.Authentication
	p auth.Principal
	r []auth.Role
}
type TestPrincipal struct {
	auth.Principal
	name string
}

func (s *TestPrincipal) Name() string {
	return s.name
}

func (s *TestAuth) Principal() auth.Principal {
	return s.p
}

func (s *TestAuth) Roles() []auth.Role {
	return s.r
}

var (
	admin = TestAuth{
		p: &TestPrincipal{name: "admin"},
		r: []auth.Role{auth.RoleAdmin},
	}
	provider = TestAuth{
		p: &TestPrincipal{name: "someprovider"},
		r: []auth.Role{auth.RoleRaceDataProvider},
	}
	tenantMgr = TestAuth{
		p: &TestPrincipal{name: "tenantManager"},
		r: []auth.Role{auth.RoleTenantManager},
	}
	trackMgr = TestAuth{
		p: &TestPrincipal{name: "trackManager"},
		r: []auth.Role{auth.RoleTrackManager},
	}
)

func TestOpa_HasPermission_Tenant(t *testing.T) {
	type args struct {
		perm Permission
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "create tenant",
			args: args{perm: PermissionCreateTenant},
			want: true,
		},
		{
			name: "delete tenant",
			args: args{perm: PermissionDeleteTenant},
			want: true,
		},
		{
			name: "update tenant",
			args: args{perm: PermissionUpdateTenant},
			want: true,
		},
		{
			name: "read tenant",
			args: args{perm: PermissionReadTenant},
			want: true,
		},
	}
	opaPE, err := NewOpaPermissionEvaluator()
	if err != nil {
		t.Errorf("NewOpaPermissionEvaluator() error = %v", err)
		return
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := opaPE.HasPermission(&tenantMgr, tt.args.perm); got != tt.want {
				t.Errorf("opaPE.HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpa_HasPermission_Track(t *testing.T) {
	type args struct {
		perm Permission
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "create track",
			args: args{perm: PermissionCreateTrack},
			want: true,
		},
		{
			name: "delete track",
			args: args{perm: PermissionDeleteTrack},
			want: true,
		},
		{
			name: "update track",
			args: args{perm: PermissionUpdateTrack},
			want: true,
		},
	}
	opaPE, err := NewOpaPermissionEvaluator()
	if err != nil {
		t.Errorf("NewOpaPermissionEvaluator() error = %v", err)
		return
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := opaPE.HasPermission(&trackMgr, tt.args.perm); got != tt.want {
				t.Errorf("opaPE.HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpa_HasObjectPermission_Provider(t *testing.T) {
	type args struct {
		perm     Permission
		objOwner string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// Note: if we omit the objOwner, the result should be the same as hasPermission
		{
			name: "register event",
			args: args{perm: PermissionRegisterEvent},
			want: true,
		},
		{
			name: "unregister event",
			args: args{
				perm:     PermissionUnregisterEvent,
				objOwner: provider.Principal().Name(),
			},
			want: true,
		},

		{
			name: "unregister foreign event",
			args: args{perm: PermissionUnregisterEvent, objOwner: "other"},
			want: false,
		},
		{
			name: "post racedata event",
			args: args{
				perm:     PermissionPostRacedata,
				objOwner: provider.Principal().Name(),
			},
			want: true,
		},
		{
			name: "post racedata foreign event",
			args: args{perm: PermissionPostRacedata, objOwner: "other"},
			want: false,
		},
		{
			name: "update event",
			args: args{
				perm:     PermissionUpdateEvent,
				objOwner: provider.Principal().Name(),
			},
			want: true,
		},
		{
			name: "delete event",
			args: args{
				perm:     PermissionDeleteEvent,
				objOwner: provider.Principal().Name(),
			},
			want: true,
		},
	}
	opaPE, err := NewOpaPermissionEvaluator()
	if err != nil {
		t.Errorf("NewOpaPermissionEvaluator() error = %v", err)
		return
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := opaPE.HasObjectPermission(
				&provider,
				tt.args.perm,
				tt.args.objOwner); got != tt.want {
				t.Errorf("opaPE.HasObjectPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpa_HasPermission_Admin(t *testing.T) {
	type args struct {
		perm Permission
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// Note: we use one permission for each role
		{
			name: "register event",
			args: args{perm: PermissionRegisterEvent},
			want: true,
		},
		{
			name: "create tenant",
			args: args{perm: PermissionCreateTenant},
			want: true,
		},
		{
			name: "create track",
			args: args{perm: PermissionCreateTrack},
			want: true,
		},
		{
			name: "unregister all events",
			args: args{perm: PermissionAdminUnregisterAllEvents},
			want: true,
		},
	}
	opaPE, err := NewOpaPermissionEvaluator()
	if err != nil {
		t.Errorf("NewOpaPermissionEvaluator() error = %v", err)
		return
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := opaPE.HasPermission(
				&admin,
				tt.args.perm); got != tt.want {
				t.Errorf("opaPE.HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}
