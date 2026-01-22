//go:build linux

package pamwrap

import (
	"net/http"

	"github.com/CARTAvis/go-carta/pkg/config"
	authpam "github.com/CARTAvis/go-carta/services/carta-ctl/internal/auth/pam"
)

func newImpl(cfg config.PAMConfig) (Authenticator, error) {
	return authpam.New(cfg), nil
}

func setSessionCookieImpl(w http.ResponseWriter, username string) error {
	return authpam.SetPAMSessionCookie(w, username)
}
