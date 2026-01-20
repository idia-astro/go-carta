//go:build linux
// +build linux

package pam

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/msteinert/pam"

	"github.com/idia-astro/go-carta/pkg/config"
	"github.com/idia-astro/go-carta/services/controller/internal/auth"
)

type PAMAuthenticator struct {
	serviceName string
}

func New(cfg config.PAMConfig) *PAMAuthenticator {
	return &PAMAuthenticator{serviceName: cfg.ServiceName}
}

// AuthenticateCredentials runs PAM for an explicit username/password pair.
// This is used by the HTML login form handler.
func (p *PAMAuthenticator) AuthenticateCredentials(ctx context.Context, username, password string) (*auth.User, error) {
	t, err := pam.StartFunc(p.serviceName, username,
		func(s pam.Style, msg string) (string, error) {
			switch s {
			case pam.PromptEchoOff, pam.PromptEchoOn:
				return password, nil
			default:
				return "", nil
			}
		},
	)
	if err != nil {
		return nil, err
	}

	if err := t.Authenticate(0); err != nil {
		return nil, err
	}

	// You can look up UID/groups here if you want.
	return &auth.User{
		Username: username,
		Source:   auth.SourcePAM,
		Claims:   map[string]any{},
	}, nil
}

// AuthenticateHTTP implements the auth.Authenticator interface.
// It checks for a valid session cookie; if missing/invalid, redirects to /pam-login.
func (p *PAMAuthenticator) AuthenticateHTTP(w http.ResponseWriter, r *http.Request) (*auth.User, error) {
	// 1. Check session cookie
	if c, err := r.Cookie("carta_session"); err == nil {
		username, err := auth.VerifySessionToken(c.Value)
		if err == nil {
			return &auth.User{
				Username: username,
				Source:   auth.SourcePAM,
				Claims:   map[string]any{},
			}, nil
		}
		log.Printf("PAM session cookie invalid: %v", err)
	}

	// 2. No valid session → redirect to login page
	// Avoid infinite loop: don't redirect /pam-login to itself.
	if r.URL.Path != "/pam-login" {
		http.Redirect(w, r, "/pam-login", http.StatusFound)
	} else {
		// If somehow we get here for /pam-login itself, just let the handler deal with it.
		w.WriteHeader(http.StatusUnauthorized)
	}
	return nil, errors.New("no valid PAM session")
}

// Helper: sets session cookie for a user.
func SetPAMSessionCookie(w http.ResponseWriter, username string) error {
	expiry := time.Now().Add(8 * time.Hour)
	token, err := auth.GenerateSessionToken(username, expiry)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "carta_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // set true if serving over HTTPS
		SameSite: http.SameSiteLaxMode,
		Expires:  expiry,
	})
	return nil
}
