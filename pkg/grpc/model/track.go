package model

import tenantv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/tenant/v1"

type Tenant struct {
	Id     uint32
	ApiKey string
	Tenant *tenantv1.Tenant
}
