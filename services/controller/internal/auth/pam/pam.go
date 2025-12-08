// internal/auth/pam/pam.go
package pam

import (
	"errors"
	"net/http"

	"github.com/msteinert/pam"

	"idia-astro/go-carta/services/controller/internal/auth"
	"idia-astro/go-carta/services/controller/internal/config"
)

type PAMAuthenticator struct {
	serviceName string
}

func New(cfg config.PAMConfig) *PAMAuthenticator {
	return &PAMAuthenticator{serviceName: cfg.ServiceName}
}

func (p *PAMAuthenticator) AuthenticateHTTP(w http.ResponseWriter, r *http.Request) (*auth.User, error) {
	// Simple HTTP Basic example; you can swap for JSON POST if you prefer
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="carta"`)
		w.WriteHeader(http.StatusUnauthorized)
		return nil, errors.New("missing basic auth")
	}

	t, err := pam.StartFunc(p.serviceName, username,
		func(s pam.Style, msg string) (string, error) {
			switch s {
			case pam.PromptEchoOff:
				return password, nil
			case pam.PromptEchoOn:
				return password, nil
			default:
				return "", nil
			}
		},
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return nil, err
	}

	if err := t.Authenticate(0); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return nil, err
	}

	// You *can* fetch groups via OS calls if you need.
	user := &auth.User{
		Username: username,
		Source:   auth.SourcePAM,
		Claims:   map[string]any{},
	}
	return user, nil
}
