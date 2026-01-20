//go:build !linux
// +build !linux

package pamwrap

import (
	"net/http"

	"github.com/CARTAvis/go-carta/services/controller/internal/config"
)

func newImpl(cfg config.PAMConfig) (Authenticator, error) {
	return nil, ErrUnsupported
}

func setSessionCookieImpl(w http.ResponseWriter, username string) error {
	return ErrUnsupported
}
