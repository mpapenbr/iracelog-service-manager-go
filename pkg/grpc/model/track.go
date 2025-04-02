package model

import tenantv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/tenant/v1"

type Tenant struct {
	ID     uint32
	APIKey string
	Tenant *tenantv1.Tenant
}
