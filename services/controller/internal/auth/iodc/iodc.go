// internal/auth/oidc/oidc.go
package oidc

import (
	"context"
	"errors"
	"net/http"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"idia-astro/go-carta/services/controller/internal/auth"
	"idia-astro/go-carta/services/controller/internal/config"
)

type OIDCAuthenticator struct {
	provider *gooidc.Provider
	verifier *gooidc.IDTokenVerifier
	oauthCfg *oauth2.Config
}

func New(cfg config.OIDCConfig) *OIDCAuthenticator {
	ctx := context.Background()
	provider, err := gooidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		panic(err) // or handle gracefully
	}

	verifier := provider.Verifier(&gooidc.Config{
		ClientID: cfg.ClientID,
	})

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  cfg.RedirectURL,
		Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
	}

	return &OIDCAuthenticator{
		provider: provider,
		verifier: verifier,
		oauthCfg: oauthCfg,
	}
}

func (a *OIDCAuthenticator) AuthenticateHTTP(w http.ResponseWriter, r *http.Request) (*auth.User, error) {
	// Simplest pattern: expect a Bearer token (e.g. from frontend or reverse proxy)
	hdr := r.Header.Get("Authorization")
	if hdr == "" || !strings.HasPrefix(strings.ToLower(hdr), "bearer ") {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, errors.New("missing bearer token")
	}
	rawIDToken := strings.TrimSpace(hdr[len("Bearer "):])

	ctx := r.Context()
	idToken, err := a.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, err
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, err
	}

	username := ""
	if v, ok := claims["preferred_username"].(string); ok {
		username = v
	} else if v, ok := claims["sub"].(string); ok {
		username = v
	}

	user := &auth.User{
		Username: username,
		Source:   auth.SourceOIDC,
		Claims:   claims,
	}
	return user, nil
}
