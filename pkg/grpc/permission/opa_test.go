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
	s []auth.ScopedRole
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

func (s *TestAuth) ScopedRoles() []auth.ScopedRole {
	return s.s
}

var (
	admin = TestAuth{
		p: &TestPrincipal{name: "admin"},
		r: []auth.Role{auth.RoleAdmin},
	}
	provider = TestAuth{
		p: &TestPrincipal{name: "someprovider"},
		r: []auth.Role{},
		s: []auth.ScopedRole{
			{Role: auth.RoleRaceDataProvider, Scopes: []string{"someprovider"}},
		},
	}
	editor = TestAuth{
		p: &TestPrincipal{name: "someeditor"},
		r: []auth.Role{},
		s: []auth.ScopedRole{
			{Role: auth.RoleEditor, Scopes: []string{"someeditor"}},
		},
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
		{
			name: "register event",
			args: args{perm: PermissionRegisterEvent, objOwner: provider.Principal().Name()},
			want: true,
		},
		{
			name: "register event not owner",
			args: args{perm: PermissionRegisterEvent, objOwner: "other"},
			want: false,
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
			name: "update event not owner",
			args: args{
				perm:     PermissionUpdateEvent,
				objOwner: "other",
			},
			want: false,
		},
		{
			name: "delete event",
			args: args{
				perm:     PermissionDeleteEvent,
				objOwner: provider.Principal().Name(),
			},
			want: true,
		},
		{
			name: "delete event not owner",
			args: args{
				perm:     PermissionDeleteEvent,
				objOwner: "other",
			},
			want: false,
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

func TestOpa_HasObjectPermission_Editor(t *testing.T) {
	type args struct {
		perm     Permission
		objOwner string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "update event",
			args: args{
				perm:     PermissionUpdateEvent,
				objOwner: editor.Principal().Name(),
			},
			want: true,
		},
		{
			name: "update event not owner",
			args: args{
				perm:     PermissionUpdateEvent,
				objOwner: "other",
			},
			want: false,
		},
		{
			name: "delete event",
			args: args{
				perm:     PermissionDeleteEvent,
				objOwner: editor.Principal().Name(),
			},
			want: true,
		},
		{
			name: "delete event not owner",
			args: args{
				perm:     PermissionDeleteEvent,
				objOwner: "other",
			},
			want: false,
		},
		{
			name: "register event not allowed",
			args: args{
				perm:     PermissionRegisterEvent,
				objOwner: editor.Principal().Name(),
			},
			want: false,
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
				&editor,
				tt.args.perm,
				tt.args.objOwner); got != tt.want {
				t.Errorf("opaPE.HasObjectPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpa_HasObjectPermission_Mixed(t *testing.T) {
	type args struct {
		testAuth TestAuth
		perm     Permission
		objOwner string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "register event not allowed",
			args: args{
				testAuth: TestAuth{
					p: &TestPrincipal{name: "mixed"}, // name does not matter here
					r: []auth.Role{},
					s: []auth.ScopedRole{
						{Role: auth.RoleEditor, Scopes: []string{"A"}},
						{Role: auth.RoleRaceDataProvider, Scopes: []string{"B"}},
					},
				},
				perm:     PermissionRegisterEvent,
				objOwner: "A",
			},
			want: false,
		},
		{
			name: "update event allowed",
			args: args{
				testAuth: TestAuth{
					p: &TestPrincipal{name: "mixed"}, // name does not matter here
					r: []auth.Role{},
					s: []auth.ScopedRole{
						{Role: auth.RoleEditor, Scopes: []string{"A"}},
						{Role: auth.RoleRaceDataProvider, Scopes: []string{"B"}},
					},
				},
				perm:     PermissionUpdateEvent,
				objOwner: "A",
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
				&tt.args.testAuth,
				tt.args.perm,
				tt.args.objOwner); got != tt.want {
				t.Errorf("opaPE.HasObjectPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}
