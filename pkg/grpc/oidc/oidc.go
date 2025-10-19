package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

//nolint:lll,tagliatelle //readability,external API
type (
	OIDCParam struct {
		IssuerURL    string
		ClientID     string
		ClientSecret string
		CallbackURL  string
	}
	PendingAuthState struct {
		State             string    `json:"state"`
		CodeVerifier      string    `json:"code_verifier"`
		ClientRedirectURI string    `json:"client_redirect_uri"`
		CreatedAt         time.Time `json:"created_at"`
	}
	WellKnownOpenIDConfig struct {
		Issuer                            string   `json:"issuer"`
		AuthorizationEndpoint             string   `json:"authorization_endpoint"`
		TokenEndpoint                     string   `json:"token_endpoint"`
		UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
		JwksURI                           string   `json:"jwks_uri"`
		ResponseTypesSupported            []string `json:"response_types_supported"`
		SubjectTypesSupported             []string `json:"subject_types_supported"`
		IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
		ScopesSupported                   []string `json:"scopes_supported"`
		TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
		ClaimsSupported                   []string `json:"claims_supported"`
		GrantTypesSupported               []string `json:"grant_types_supported"`
		EndSessionEndpoint                string   `json:"end_session_endpoint"`
	}

	// used to hold PendingLogin data during authorization code flow
	PendingAuthStateCache interface {
		Save(pl *PendingAuthState) error
		Get(state string) (*PendingAuthState, error)
		Delete(state string) error
	}
)

var ErrStateNotFound = errors.New("state not found")

// used to get additional endpoints that oidc provider does not provide
//
//nolint:whitespace // editor/linter issue
func GetWellKnownConfig(issuerURL string) (
	*WellKnownOpenIDConfig,
	error,
) {
	req, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("%s/%s",
			strings.TrimRight(issuerURL, "/"), ".well-known/openid-configuration"),
		http.NoBody)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 status code: %d", resp.StatusCode)
	}
	var config WellKnownOpenIDConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode well-known configuration: %w", err)
	}

	return &config, nil
}
