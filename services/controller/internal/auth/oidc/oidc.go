// services/controller/internal/auth/oidc/oidc.go
package oidc

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/idia-astro/go-carta/services/controller/internal/auth"
	"github.com/idia-astro/go-carta/services/controller/internal/config"
)

const sessionCookieName = "carta_oidc"

type OIDCAuthenticator struct {
	provider *gooidc.Provider
	verifier *gooidc.IDTokenVerifier
	oauth2   *oauth2.Config
}

func New(cfg config.OIDCConfig) *OIDCAuthenticator {
	ctx := context.Background()

	provider, err := gooidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		log.Panicf("OIDC: failed to create provider for %q: %v", cfg.IssuerURL, err)
	}

	oidcCfg := &gooidc.Config{
		ClientID: cfg.ClientID,
	}

	verifier := provider.Verifier(oidcCfg)

	oauth2cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  cfg.RedirectURL,
		Scopes: []string{
			gooidc.ScopeOpenID,
			"profile",
			"email",
		},
	}

	return &OIDCAuthenticator{
		provider: provider,
		verifier: verifier,
		oauth2:   oauth2cfg,
	}
}

// AuthenticateHTTP implements auth.Authenticator.
//
// Behaviour for browser flows:
//
//  1. If path is /oidc/login or /oidc/callback → we *don't* handle here
//     (those handlers are wired separately).
//  2. Try to authenticate from the session cookie (carta_oidc).
//  3. If no valid cookie and no Bearer token → redirect to /oidc/login.
//
// Behaviour for API clients:
//
//  1. If there is an Authorization: Bearer <token> header, verify it.
//  2. If valid → return user; if invalid → 401 via caller.
func (o *OIDCAuthenticator) AuthenticateHTTP(w http.ResponseWriter, r *http.Request) (*auth.User, error) {
	// Allow the OIDC endpoints themselves to run without auth.
	if r.URL.Path == "/oidc/login" || r.URL.Path == "/oidc/callback" {
		return nil, fmt.Errorf("oidc endpoint passthrough")
	}

	ctx := r.Context()

	// 1. Try session cookie first (browser flow)
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		if user, err := o.verifySessionCookie(ctx, c.Value); err == nil {
			return user, nil
		} else {
			log.Printf("OIDC: invalid session cookie: %v", err)
		}
	}

	// 2. Try Bearer token (API clients)
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		raw := strings.TrimSpace(authHeader[len("bearer "):])
		if user, err := o.verifyRawToken(ctx, raw); err == nil {
			return user, nil
		} else {
			log.Printf("OIDC: bearer token verification failed: %v", err)
		}
	}

	// 3. No session → redirect browser to login
	http.Redirect(w, r, "/oidc/login", http.StatusFound)
	return nil, fmt.Errorf("no OIDC session")
}

// LoginHandler redirects the user to Keycloak's authorization endpoint.
func (o *OIDCAuthenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// You can generate and store a proper state value; for now use a fixed one
	// or something simple. In production, use a random per-session value.
	state := "carta-state"

	url := o.oauth2.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}

// CallbackHandler handles the redirect from Keycloak, exchanges the code
// for tokens, validates the ID token, sets the session cookie, and redirects
// back to the main UI (/).
func (o *OIDCAuthenticator) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		http.Error(w, fmt.Sprintf("OIDC error: %s (%s)", errParam, desc), http.StatusUnauthorized)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing code", http.StatusBadRequest)
		return
	}

	// TODO: validate 'state' if you generate a random one in LoginHandler

	oauth2Token, err := o.oauth2.Exchange(ctx, code)
	if err != nil {
		log.Printf("OIDC: code exchange failed: %v", err)
		http.Error(w, "Code exchange failed", http.StatusUnauthorized)
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		log.Printf("OIDC: no id_token in token response")
		http.Error(w, "No id_token in token response", http.StatusUnauthorized)
		return
	}

	// Verify and extract user (mainly for logging / sanity)
	user, err := o.verifyRawToken(ctx, rawIDToken)
	if err != nil {
		log.Printf("OIDC: id_token verification failed: %v", err)
		http.Error(w, "Invalid id_token", http.StatusUnauthorized)
		return
	}

	// Store the raw ID token in a session cookie. It's already signed by Keycloak;
	// we will re-verify it on each request via verifySessionCookie.
	expiry := time.Now().Add(8 * time.Hour)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    rawIDToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // set true if you serve over HTTPS
		SameSite: http.SameSiteLaxMode,
		Expires:  expiry,
	})

	log.Printf("OIDC: login successful for user %s", user.Username)

	http.Redirect(w, r, "/", http.StatusFound)
}

// verifySessionCookie takes the cookie value (raw ID token) and verifies it.
func (o *OIDCAuthenticator) verifySessionCookie(ctx context.Context, rawIDToken string) (*auth.User, error) {
	return o.verifyRawToken(ctx, rawIDToken)
}

// verifyRawToken verifies an ID token string and builds an auth.User from it.
func (o *OIDCAuthenticator) verifyRawToken(ctx context.Context, raw string) (*auth.User, error) {
	idToken, err := o.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("verifyRawToken: %w", err)
	}

	var claims struct {
		Email         string `json:"email"`
		PreferredName string `json:"preferred_username"`
		Name          string `json:"name"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}

	username := claims.PreferredName
	if username == "" {
		username = claims.Email
	}
	if username == "" {
		username = claims.Name
	}
	if username == "" {
		username = "unknown"
	}

	// If you want all claims, you can unmarshal into a generic map as well:
	var allClaims map[string]any
	if err := idToken.Claims(&allClaims); err != nil {
		allClaims = map[string]any{}
	}

	// Be careful not to blow up logs; but store claims in the User struct.
	user := &auth.User{
		Username: username,
		Source:   auth.SourceOIDC,
		Claims:   allClaims,
	}

	return user, nil
}
