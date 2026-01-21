package pamwrap

import (
	"context"
	"errors"
	"net/http"

	"github.com/CARTAvis/go-carta/pkg/config"
	"github.com/CARTAvis/go-carta/services/carta-ctl/internal/auth"
)

var ErrUnsupported = errors.New("PAM auth is only supported on Linux")

// Authenticator is what main.go needs.
type Authenticator interface {
	auth.Authenticator
	AuthenticateCredentials(ctx context.Context, username, password string) (*auth.User, error)
}

func New(cfg config.PAMConfig) (Authenticator, error) {
	return newImpl(cfg)
}

func SetSessionCookie(w http.ResponseWriter, username string) error {
	return setSessionCookieImpl(w, username)
}
