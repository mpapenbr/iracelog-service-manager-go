package oauth2

import (
	"time"

	"golang.org/x/oauth2"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
)

type (
	//nolint:tagliatelle // external API
	sessionRawData struct {
		ID           string            `json:"id"`
		Name         string            `json:"name"`
		Roles        []auth.Role       `json:"roles"`
		ScopedRoles  []auth.ScopedRole `json:"scoped_roles"`
		Token        *oauth2.Token     `json:"token"`
		ExtraIDToken string            `json:"extra_id_token"` // needs extra handling
		LastAccessed time.Time         `json:"last_accessed"`
		UserID       string            `json:"user_id"`
		TenantID     uint32            `json:"tenant_id"`
		LastSync     time.Time         `json:"last_sync"` // Last synchronization time
	}
)
