package permission

import (
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
)

type PermissionEvaluator interface {
	HasRole(auth *auth.Authentication, role auth.Role) bool
}

func NewPermissionEvaluator() PermissionEvaluator {
	return &myPermissionEvaluator{}
}

type myPermissionEvaluator struct{}

func (*myPermissionEvaluator) HasRole(a *auth.Authentication, role auth.Role) bool {
	for _, r := range (*a).Roles() {
		if r == role {
			return true
		}
	}
	return false
}
