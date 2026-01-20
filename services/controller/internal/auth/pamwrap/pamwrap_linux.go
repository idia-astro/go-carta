//go:build linux
// +build linux

package pamwrap

import (
	"net/http"

	authpam "github.com/CARTAvis/go-carta/services/controller/internal/auth/pam"
	"github.com/CARTAvis/go-carta/services/controller/internal/config"
)

func newImpl(cfg config.PAMConfig) (Authenticator, error) {
	return authpam.New(cfg), nil
}

func setSessionCookieImpl(w http.ResponseWriter, username string) error {
	return authpam.SetPAMSessionCookie(w, username)
}
