package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
)

func Test_extractScopedRoles(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		roles []auth.Role
		want  []auth.ScopedRole
	}{
		{
			name:  "no roles",
			roles: []auth.Role{},
			want:  []auth.ScopedRole{},
		},
		{
			name:  "no scoped roles",
			roles: []auth.Role{"admin", "user"},
			want:  []auth.ScopedRole{},
		},
		{
			name:  "simple scoped roles",
			roles: []auth.Role{"editor_tenant_12", "user"},
			want: []auth.ScopedRole{
				{Role: "editor", Scopes: []string{"12"}},
			},
		},
		{
			name: "multiple scoped roles",
			roles: []auth.Role{
				"editor_tenant_12",
				"editor_tenant_34",
				"racedata-provider_tenant_56",
				"editor_tenant_78",
			},
			want: []auth.ScopedRole{
				{Role: "editor", Scopes: []string{"12", "34", "78"}},
				{Role: "racedata-provider", Scopes: []string{"56"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractScopedRoles(tt.roles)
			// TODO: update the condition below to compare got with tt.want.
			if !assert.ElementsMatch(t, got, tt.want) {
				t.Errorf("extractScopedRoles() = %v, want %v", got, tt.want)
			}
		})
	}
}
