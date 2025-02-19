package permission

import (
	"bytes"
	"context"
	_ "embed"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
)

type OpaPermissionEvaluator struct {
	// PermissionEvaluator
	r     *rego.Rego
	query rego.PreparedEvalQuery
	l     *log.Logger
}

type EvalRequest struct {
	Tenant      string      `json:"tenant"`
	Roles       []auth.Role `json:"roles"`
	Action      Permission  `json:"action"`
	ObjectOwner string      `json:"objectOwner,omitempty"`
}

//go:embed policy.rego
var policy []byte

//go:embed data.json
var data []byte

func NewOpaPermissionEvaluator() (*OpaPermissionEvaluator, error) {
	l := log.Default().Named("permission").Named("opa")
	store := inmem.NewFromReader(bytes.NewReader(data))
	r := rego.New(
		rego.Query("data.iracelog.authz.allow"),
		rego.Module("iracelog.authz", string(policy)),
		rego.Store(store),
	)
	if query, err := r.PrepareForEval(context.Background()); err != nil {
		l.Error("failed to prepare query", log.ErrorField(err))
		return nil, err
	} else {
		return &OpaPermissionEvaluator{
			r:     r,
			query: query,
			l:     l,
		}, nil
	}
}

func (x *OpaPermissionEvaluator) HasRole(a auth.Authentication, role auth.Role) bool {
	x.l.Info("HasRole", log.String("role", string(role)))
	return false
}

//nolint:whitespace // editor/linter issue
func (x *OpaPermissionEvaluator) HasPermission(
	a auth.Authentication,
	perm Permission,
) bool {
	x.l.Debug("HasPermission",
		log.String("name", a.Principal().Name()),
		log.Any("roles", a.Roles()),
		log.String("perm", string(perm)))
	req := EvalRequest{
		Tenant: a.Principal().Name(),
		Roles:  a.Roles(),
		Action: perm,
	}
	if rs, err := x.query.Eval(context.Background(), rego.EvalInput(req)); err != nil {
		log.Default().Error("HasPermission", log.ErrorField(err))
		return false
	} else {
		log.Default().Debug("res", log.Any("res", rs))
		return rs.Allowed()
	}
}

//nolint:whitespace // editor/linter issue
func (x *OpaPermissionEvaluator) HasObjectPermission(
	a auth.Authentication,
	perm Permission,
	objectOwner string,
) bool {
	x.l.Debug("HasObjectPermission",
		log.String("name", a.Principal().Name()),
		log.Any("roles", a.Roles()),
		log.String("perm", string(perm)),
		log.String("objectOwner", objectOwner))
	req := EvalRequest{
		Tenant:      a.Principal().Name(),
		Roles:       a.Roles(),
		Action:      perm,
		ObjectOwner: objectOwner,
	}
	if rs, err := x.query.Eval(context.Background(), rego.EvalInput(req)); err != nil {
		x.l.Error("HasObjectPermission", log.ErrorField(err))
		return false
	} else {
		x.l.Debug("res", log.Any("res", rs))
		return rs.Allowed()
	}
}
