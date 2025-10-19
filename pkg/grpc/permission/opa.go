package permission

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/samber/lo"

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
	Roles       []auth.Role            `json:"roles"`
	Scoped      map[auth.Role][]string `json:"scoped,omitempty"`
	Action      Permission             `json:"action"`
	ObjectOwner string                 `json:"objectOwner,omitempty"`
}

// check interface compliance
var _ PermissionEvaluator = (*OpaPermissionEvaluator)(nil)

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

//nolint:whitespace // editor/linter issue
func (ope *OpaPermissionEvaluator) HasPermission(
	a auth.Authentication,
	perm Permission,
) bool {
	ope.l.Debug("HasPermission",
		log.String("name", a.Principal().Name()),
		log.Any("roles", a.Roles()),
		log.String("perm", string(perm)))
	req := EvalRequest{
		Roles: a.Roles(),
		// Scoped: [],
		Action: perm,
	}
	if rs, err := ope.query.Eval(context.Background(), rego.EvalInput(req)); err != nil {
		ope.l.Error("HasPermission", log.ErrorField(err))
		return false
	} else {
		ope.l.Debug("res", log.Any("res", rs))
		return rs.Allowed()
	}
}

//nolint:whitespace // editor/linter issue
func (ope *OpaPermissionEvaluator) HasObjectPermission(
	a auth.Authentication,
	perm Permission,
	objectOwner string,
) bool {
	ope.l.Debug("HasObjectPermission",
		log.String("name", a.Principal().Name()),
		log.Any("roles", a.Roles()),
		log.Any("scopedRoles", a.ScopedRoles()),
		log.String("perm", string(perm)),
		log.String("objectOwner", objectOwner))

	scoped := lo.Associate(a.ScopedRoles(),
		func(sr auth.ScopedRole) (auth.Role, []string) {
			return sr.Role, sr.Scopes
		})
	req := EvalRequest{
		Roles:       a.Roles(), // needed for admin check
		Scoped:      scoped,
		Action:      perm,
		ObjectOwner: objectOwner,
	}
	if rs, err := ope.query.Eval(context.Background(), rego.EvalInput(req)); err != nil {
		ope.l.Error("HasObjectPermission", log.ErrorField(err))
		return false
	} else {
		ope.l.Debug("res", log.Any("res", rs))
		return rs.Allowed()
	}
}

//nolint:whitespace // editor/linter issue
func (ope *OpaPermissionEvaluator) HasTenantPermission(
	a auth.Authentication,
	perm Permission,
	tenantID uint32,
) bool {
	objectOwner := fmt.Sprintf("%d", tenantID)
	return ope.HasObjectPermission(a, perm, objectOwner)
}
