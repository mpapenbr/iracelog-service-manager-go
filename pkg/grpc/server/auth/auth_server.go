package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	x "buf.build/gen/go/mpapenbr/iracelog/connectrpc/go/iracelog/auth/v1/authv1connect"
	authv1 "buf.build/gen/go/mpapenbr/iracelog/protocolbuffers/go/iracelog/auth/v1"
	"connectrpc.com/connect"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/oauth2"

	"github.com/mpapenbr/iracelog-service-manager-go/log"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/auth"
	ownOidc "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/oidc"
	"github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session"
	sImpl "github.com/mpapenbr/iracelog-service-manager-go/pkg/grpc/session/impl/oauth2"
)

func NewServer(opts ...Option) *authServer {
	ret := &authServer{
		log:              log.Default().Named("grpc.auth"),
		refreshThreshold: 10 * time.Second,
	}
	for _, opt := range opts {
		opt(ret)
	}
	var err error
	ret.oidcProvider, err = oidc.NewProvider(context.Background(), ret.oidcParam.IssuerURL)
	if err != nil {
		ret.log.Fatal("failed to create oidc provider", log.ErrorField(err))
	}
	oauth2Config := &oauth2.Config{
		ClientID:     ret.oidcParam.ClientID,
		ClientSecret: ret.oidcParam.ClientSecret,
		Endpoint:     ret.oidcProvider.Endpoint(),
		RedirectURL:  ret.oidcParam.CallbackURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	ret.verifier = ret.oidcProvider.Verifier(&oidc.Config{ClientID: oauth2Config.ClientID})
	ret.oauth2Config = oauth2Config
	ret.wellKnown, err = ownOidc.GetWellKnownConfig(ret.oidcParam.IssuerURL)
	if err != nil {
		ret.log.Warn("failed to collect additional oidc endpoints", log.ErrorField(err))
	}
	if ret.tracer == nil {
		ret.tracer = otel.Tracer("ism")
	}

	return ret
}

type (
	Option func(*authServer)

	//nolint:tagliatelle // external API
	KeycloakClaims struct {
		RealmAccess RealmAccess `json:"realm_access"`
	}
	RealmAccess struct {
		Roles []string `json:"roles"`
	}
)

func WithTracer(tracer trace.Tracer) Option {
	return func(srv *authServer) {
		srv.tracer = tracer
	}
}

func WithSessionStore(sessionStore session.SessionStore) Option {
	return func(srv *authServer) {
		srv.sessionStore = sessionStore
	}
}

func WithPendingAuthStateCache(arg ownOidc.PendingAuthStateCache) Option {
	return func(srv *authServer) {
		srv.pendingAuthStateCache = arg
	}
}

func WithOIDCParams(params *ownOidc.OIDCParam) Option {
	return func(srv *authServer) {
		srv.oidcParam = params
	}
}

func WithRefreshTokenThreshold(threshold time.Duration) Option {
	return func(srv *authServer) {
		srv.refreshThreshold = threshold
	}
}

var ErrAuthNotFound = errors.New("auth not found")

type (
	authServer struct {
		x.UnimplementedAuthServiceHandler

		log *log.Logger

		tracer       trace.Tracer
		loginCounter int
		sessionStore session.SessionStore
		oidcProvider *oidc.Provider
		wellKnown    *ownOidc.WellKnownOpenIDConfig
		oauth2Config *oauth2.Config
		verifier     *oidc.IDTokenVerifier

		pendingAuthStateCache ownOidc.PendingAuthStateCache
		oidcParam             *ownOidc.OIDCParam
		refreshThreshold      time.Duration
	}
)

// TODO: implement a glue component between IDP and SessionStore

//nolint:whitespace // can't make both editor and linter happy
func (s *authServer) Login(
	ctx context.Context,
	req *connect.Request[authv1.LoginRequest],
) (*connect.Response[authv1.LoginResponse], error) {
	s.log.Debug("Login called")
	state := randStringURL(32)
	verifierStr := oauth2.GenerateVerifier()

	if err := s.pendingAuthStateCache.Save(&ownOidc.PendingAuthState{
		State:             state,
		CodeVerifier:      verifierStr,
		ClientRedirectURI: req.Msg.GetRedirectUrl(),
		CreatedAt:         time.Now(),
	}); err != nil {
		s.log.Error("failed to save pending auth state", log.ErrorField(err))
		return nil, connect.NewError(connect.CodeInternal, err)

	}

	authURL := s.oauth2Config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifierStr),
	)

	resp := connect.NewResponse(&authv1.LoginResponse{
		LoginUrl: authURL,
	})

	return resp, nil
}

//nolint:whitespace // can't make both editor and linter happy
func (s *authServer) Logout(
	ctx context.Context,
	req *connect.Request[authv1.LogoutRequest],
) (*connect.Response[authv1.LogoutResponse], error) {
	s.log.Debug("Logout called")
	sessionData := session.SessionFromContext(ctx)
	if sessionData == nil {
		return nil, ErrAuthNotFound
	}
	resp := connect.NewResponse(&authv1.LogoutResponse{})
	if idToken := sImpl.GetIDToken(sessionData); idToken != "" {
		if loErr := s.oidcSessionLogout(
			idToken,
			s.wellKnown.EndSessionEndpoint); loErr != nil {
			s.log.Warn("failed to logout at identity provider", log.ErrorField(loErr))
		}
	} else {
		s.log.Warn("id_token is empty. cannot logout at identity provider",
			log.String("session_id", sessionData.ID()))
	}
	if err := s.sessionStore.Delete(sessionData.ID()); err != nil {
		s.log.Warn("failed to delete session", log.ErrorField(err))
	} else {
		s.log.Debug("deleted session", log.String("session_id", sessionData.ID()))
	}

	cookie := session.CreateCookieForSession(
		s.sessionStore.CookieName(),
		sessionData,
		s.sessionStore.Timeout())
	cookie.Value = ""
	cookie.MaxAge = -1 // delete cookie

	resp.Header().Set("Set-Cookie", cookie.String())
	return resp, nil
}

//nolint:funlen // many tasks to do here
func (s *authServer) CallbackHandler() (path string, handler http.Handler) {
	return "/auth/callback", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			s.log.Debug("CallbackHandler called")

			// need a new context here. this will also be used to refresh tokens
			// via TokenSource
			ctx := context.Background()

			code := r.URL.Query().Get("code")
			state := r.URL.Query().Get("state")
			if code == "" || state == "" {
				http.Error(w, "missing code or state", http.StatusBadRequest)
				return
			}
			pl, err := s.pendingAuthStateCache.Get(state)
			if err != nil {
				log.Warn("could not get info for pending login",
					log.String("state", state),
					log.ErrorField(err))
				http.Error(w, "invalid or expired state", http.StatusBadRequest)
				return
			}
			//nolint:errcheck // ignore error on delete
			s.pendingAuthStateCache.Delete(state)
			token, err := s.oauth2Config.Exchange(
				ctx,
				code,
				oauth2.VerifierOption(pl.CodeVerifier))
			if err != nil {
				s.log.Warn("failed to exchange token", log.ErrorField(err))
				http.Error(w, "failed to exchange token", http.StatusInternalServerError)
				return
			}

			rawIDToken, ok := token.Extra("id_token").(string)
			if !ok {
				s.log.Warn("no id_token in token response", log.ErrorField(err))
				http.Error(w, "no id_token in token response", http.StatusInternalServerError)
				return
			}

			idToken, err := s.verifier.Verify(ctx, rawIDToken)
			if err != nil {
				s.log.Warn("could not verify id_token", log.ErrorField(err))
				http.Error(w, "could not verify id_token", http.StatusInternalServerError)
				return
			}

			var claims map[string]any
			if err = idToken.Claims(&claims); err != nil {
				http.Error(w, "claims parse failed: "+err.Error(), http.StatusInternalServerError)
				return
			}

			log.Debug("id token claims", log.Any("claims", claims))

			tokenSource := s.oauth2Config.TokenSource(ctx, token)
			uInfo, err := s.oidcProvider.UserInfo(ctx, tokenSource)
			if err != nil {
				s.log.Warn("failed to get userinfo", log.ErrorField(err))
				http.Error(w, "failed to get userinfo", http.StatusInternalServerError)
				return
			}

			log.Debug("user info", log.Any("user_info", uInfo))
			s.loginCounter++
			// add a small buffer to token expiry. when sessionStore kicks in to
			// refresh the token token store will also think it needs to be refreshed
			early := oauth2.ReuseTokenSourceWithExpiry(
				token,
				tokenSource,
				2*time.Second+s.refreshThreshold)
			sessionData := s.buildSession(token, rawIDToken, early, claims)

			err = s.sessionStore.Save(sessionData)
			if err != nil {
				s.log.Warn("failed to save session", log.ErrorField(err))
			}
			w.Header().Set("Set-Cookie", session.CreateCookieForSession(
				s.sessionStore.CookieName(),
				sessionData,
				s.sessionStore.Timeout()).String())
			http.Redirect(w, r, pl.ClientRedirectURI, http.StatusFound)
		})
}

// used to get additional endpoints that oidc provider does not provide
//
//nolint:whitespace // editor/linter issue
func (s *authServer) oidcSessionLogout(rawIDTokenStr, logoutURL string) error {
	values := url.Values{}

	values.Set("id_token_hint", rawIDTokenStr)
	req, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		logoutURL,
		strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.oauth2Config.ClientID, s.oauth2Config.ClientSecret)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.log.Warn("failed to perform logout request", log.ErrorField(err))
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.log.Warn("logout responsed with non-200 status",
			log.Int("status_code", resp.StatusCode))
		return fmt.Errorf("non-200 status code: %d", resp.StatusCode)
	}
	return nil
}

//nolint:whitespace // editor/linter issue
func (s *authServer) buildSession(
	token *oauth2.Token,
	idToken string,
	tokenSource oauth2.TokenSource,
	claims map[string]any,
) session.Session {
	extraClaims, err := s.extractKeycloakClaims(token.AccessToken)
	if err != nil {
		s.log.Warn("failed to extract keycloak claims", log.ErrorField(err))
		extraClaims = &KeycloakClaims{}
	}

	sessionRoles := lo.Map(extraClaims.RealmAccess.Roles,
		func(role string, _ int) auth.Role {
			return auth.Role(role)
		})
	sessionID := lo.RandomString(40, lo.LettersCharset)

	//nolint:gocritic // keep it for debugging
	// sessionID :=  fmt.Sprintf("dummy-%d", s.loginCounter)
	ret := sImpl.NewSession(sessionID,
		sImpl.WithSessionName(fmt.Sprintf("%s", claims["name"])),
		sImpl.WithSessionRoles(lo.Filter(sessionRoles,
			func(r auth.Role, _ int) bool {
				return lo.Contains(auth.GlobalRoles, r)
			})),
		sImpl.WithSessionScopedRoles(extractScopedRoles(sessionRoles)),
		sImpl.WithSessionUserID(fmt.Sprintf("%s", claims["sub"])),
		sImpl.WithSessionTokenSource(tokenSource),
		sImpl.WithSessionIDToken(idToken),
	)

	return ret
}

//nolint:whitespace // editor/linter issue
func (s *authServer) extractKeycloakClaims(accessToken string) (
	ret *KeycloakClaims,
	err error,
) {
	parts := strings.Split(accessToken, ".")
	if len(parts) < 3 {
		s.log.Warn("invalid access token format")
		return nil, fmt.Errorf("invalid access token format")
	}
	var payload []byte
	payload, err = base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	claims := KeycloakClaims{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		s.log.Warn("failed to unmarshal access token claims", log.ErrorField(err))
		return nil, fmt.Errorf("failed to unmarshal access token claims: %w", err)
	}
	return &claims, nil
}

func randStringURL(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func extractScopedRoles(roles []auth.Role) []auth.ScopedRole {
	rolePattern := regexp.MustCompile(`^(racedata-provider|editor)_tenant_(\d+)$`)
	scopedRolesMap := map[auth.Role][]string{}
	for _, r := range roles {
		if matches := rolePattern.FindStringSubmatch(string(r)); len(matches) == 3 {
			scopedRolesMap[auth.Role(matches[1])] = append(
				scopedRolesMap[auth.Role(matches[1])],
				matches[2],
			)
		}
	}
	return lo.MapToSlice(scopedRolesMap,
		func(role auth.Role, scopes []string) auth.ScopedRole {
			return auth.ScopedRole{
				Role:   role,
				Scopes: scopes,
			}
		},
	)
}
