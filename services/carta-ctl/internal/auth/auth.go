// services/carta-ctl/internal/auth/auth.go
package auth

import (
	"net/http"
)

type Source string

const (
	SourcePAM  Source = "pam"
	SourceOIDC Source = "oidc"
)

type User struct {
	Username string
	UID      string
	Groups   []string
	Source   Source
	Claims   map[string]any
}

type Authenticator interface {
	AuthenticateHTTP(w http.ResponseWriter, r *http.Request) (*User, error)
}

// NoopAuthenticator – used when authMode=none
type NoopAuthenticator struct{}

func (NoopAuthenticator) AuthenticateHTTP(w http.ResponseWriter, r *http.Request) (*User, error) {
	// anonymous user
	return &User{
		Username: "anonymous",
		Source:   "",
		Claims:   map[string]any{},
	}, nil
}

// Optional: Multi authenticator (if you later want PAM+OIDC)
type MultiAuthenticator struct {
	backends []Authenticator
}

func Multi(backends ...Authenticator) MultiAuthenticator {
	return MultiAuthenticator{backends: backends}
}

func (m MultiAuthenticator) AuthenticateHTTP(w http.ResponseWriter, r *http.Request) (*User, error) {
	var lastErr error
	for _, b := range m.backends {
		u, err := b.AuthenticateHTTP(w, r)
		if err == nil {
			return u, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
