package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	helpers "github.com/idia-astro/go-carta/pkg/shared"
	"github.com/idia-astro/go-carta/services/controller/internal/config"
	"github.com/idia-astro/go-carta/services/controller/internal/session"

	"github.com/idia-astro/go-carta/services/controller/internal/auth"
	authoidc "github.com/idia-astro/go-carta/services/controller/internal/auth/oidc"
	pamwrap "github.com/idia-astro/go-carta/services/controller/internal/auth/pamwrap"
)

var (
	runtimeSpawnerAddress string
	runtimeBaseFolder     string
	pamAuth               pamwrap.Authenticator
)

var upgrader = websocket.Upgrader{
	// Ignore Origin header
	CheckOrigin: func(r *http.Request) bool {
		slog.Debug("Upgrading WebSocket connection", "origin", r.Header.Get("Origin"))
		return true
	},
}

// spaHandler serves static files if they exist, otherwise falls back to index.html
type spaHandler struct {
	root string
	fs   http.Handler
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// We can create a sub-logger for this handler from the default handler
	logger := slog.With("service", "spaHandler")
	// If this is a WebSocket upgrade (e.g. ws://localhost:8081), hand it to wsHandler
	logger.Debug("Received request", "path", r.URL.Path)
	if websocket.IsWebSocketUpgrade(r) {
		logger.Debug("Upgrading to WebSocket", "path", r.URL.Path)
		wsHandler(w, r)
		return
	}

	logger.Debug("Serving HTTP request", "path", r.URL.Path)
	// Clean and resolve requested path
	path := r.URL.Path
	if path == "" || path == "/" {
		logger.Debug("Serving root, returning index.html")
		logger.Debug("Serving root", "path", filepath.Join(h.root, "index.html"))
		http.ServeFile(w, r, filepath.Join(h.root, "index.html"))
		return
	}

	// Map URL path to filesystem path
	fullPath := filepath.Join(h.root, filepath.Clean(path))

	// If the file exists and is not a directory, serve it
	if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
		h.fs.ServeHTTP(w, r)
		return
	}

	// For everything else (including React routes), serve index.html
	http.ServeFile(w, r, filepath.Join(h.root, "index.html"))
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	slog.Debug("Handling WebSocket connection", "remoteAddr", r.RemoteAddr)
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Problem with HTTP upgrade", "error", err)
		return
	}
	defer helpers.CloseOrLog(c)

	user, _ := r.Context().Value(session.UserContextKey).(*auth.User)

	s := session.NewSession(c, runtimeSpawnerAddress, runtimeBaseFolder, user)
	slog.Info("Created new session", "user", user)

	// Send messages back to client through websocket
	s.HandleConnection()

	// Close worker on exit if it exists
	defer s.HandleDisconnect()

	// Basic handler based on gorilla/websocket example
	for {
		messageType, message, err := c.ReadMessage()
		if err != nil {
			slog.Error("Error reading message", "error", err)
			break
		}

		// Ping/pong sequence
		if messageType == websocket.TextMessage && string(message) == "PING" {
			slog.Debug("Received PING from client, sending PONG")
			err := c.WriteMessage(websocket.TextMessage, []byte("PONG"))
			if err != nil {
				slog.Error("Failed to send pong message", "error", err)
			}
			continue
		}

		// Ignore all other non-binary messages
		if messageType != websocket.BinaryMessage {
			slog.Warn("Ignoring non-binary message", "type", messageType, "message", message)
			continue
		}

		go func() {
			err := s.HandleMessage(message)
			if err != nil {
				slog.Warn("Failed to handle message", "error", err)
			}
		}()
	}

	// defer should shut down the worker afterwards
	slog.Info("Client disconnected")
}

func withAuth(a auth.Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := a.AuthenticateHTTP(w, r)
		if err != nil {
			// Expected when we just redirected to /oidc/login
			if strings.Contains(err.Error(), "no OIDC session") {
				// optional: log at debug level instead
				slog.Debug("Redirecting to OIDC login")
				return
			}
			slog.Error("Auth failed", "error", err)
			return
		}

		// Attach user to context
		ctx := context.WithValue(r.Context(), session.UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func pamLoginHandler(p pamwrap.Authenticator) http.Handler {
	slog.Info("Setting up PAM login handler")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, `
<!DOCTYPE html>
<html>
  <head><title>CARTA Login</title></head>
  <body>
    <h2>CARTA Login (PAM)</h2>
    <form method="POST">
      <label>Username: <input name="username" /></label><br/>
      <label>Password: <input type="password" name="password" /></label><br/>
      <button type="submit">Login</button>
    </form>
  </body>
</html>
`)
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Bad form", http.StatusBadRequest)
				return
			}
			username := r.Form.Get("username")
			password := r.Form.Get("password")
			if username == "" || password == "" {
				http.Error(w, "Missing username or password", http.StatusBadRequest)
				return
			}

			user, err := p.AuthenticateCredentials(r.Context(), username, password)
			if err != nil {
				slog.Error("PAM login failed", "username", username, "error", err)
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}

			if err := pamwrap.SetSessionCookie(w, user.Username); err != nil {
				slog.Error("Failed to set PAM session cookie", "username", username, "error", err)
				http.Error(w, "Session error", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/", http.StatusFound)
		default:
			slog.Warn("Ignoring unsupported method", "method", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

var oidcAuth *authoidc.OIDCAuthenticator

func main() {
	logger := helpers.NewLogger("controller", "debug")
	slog.SetDefault(logger)

	id := uuid.New()
	slog.Info("Starting controller", "uuid", id.String())

	cfg := config.Config{}
	flag.IntVar(&cfg.Port, "port", 8081, "TCP server port")
	flag.StringVar(&cfg.Hostname, "hostname", "", "Hostname to listen on")
	flag.StringVar(&cfg.SpawnerAddress, "spawnerAddress", "http://localhost:8080", "Address of the process spawner")
	flag.StringVar(&cfg.BaseFolder, "baseFolder", "", "Base folder for data")
	flag.StringVar(&cfg.FrontendDir, "frontendDir", "", "Directory with carta_frontend")

	flag.StringVar((*string)(&cfg.AuthMode), "authMode", "none", "Authentication mode: none|pam|oidc|both")

	flag.StringVar(&cfg.PAM.ServiceName, "pamService", "carta", "PAM service name (for pam authMode)")

	flag.StringVar(&cfg.OIDC.IssuerURL, "oidcIssuer", "", "OIDC issuer URL (e.g. https://keycloak.example.com/realms/xyz)")
	flag.StringVar(&cfg.OIDC.ClientID, "oidcClientID", "", "OIDC client ID")
	flag.StringVar(&cfg.OIDC.ClientSecret, "oidcClientSecret", "", "OIDC client secret")
	flag.StringVar(&cfg.OIDC.RedirectURL, "oidcRedirectURL", "", "OIDC redirect/callback URL")

	flag.Parse()

	runtimeSpawnerAddress = cfg.SpawnerAddress
	runtimeBaseFolder = cfg.BaseFolder

	var authenticator auth.Authenticator

	slog.Debug("Configuring auth", "authMode", cfg.AuthMode)

	switch cfg.AuthMode {
	case config.AuthNone:
		authenticator = auth.NoopAuthenticator{}
	case config.AuthPAM:
		p, err := pamwrap.New(cfg.PAM)
		if err != nil {
			slog.Error("PAM is not available on this platform", "error", err)
			os.Exit(1)
		}
		pamAuth = p
		authenticator = p

	case config.AuthOIDC:
		o := authoidc.New(cfg.OIDC)
		oidcAuth = o
		authenticator = o

	case config.AuthBoth:
		p, err := pamwrap.New(cfg.PAM)
		if err != nil {
			slog.Error("Auth mode 'both' requires PAM, but PAM is not available on this platform", "error", err)
			os.Exit(1)
		}
		pamAuth = p
		authenticator = auth.Multi(
			p,
			authoidc.New(cfg.OIDC),
		)

	default:
		slog.Error("Unknown config option", "authMod", cfg.AuthMode)
		os.Exit(1)
	}
	// Default baseFolder to $HOME if unset
	if len(strings.TrimSpace(cfg.BaseFolder)) == 0 {
		dirname, err := os.UserHomeDir()
		if err != nil {
			dirname = "/"
		}
		if err := flag.Set("baseFolder", dirname); err != nil {
			slog.Error("Failed to set baseFolder", "error", err, "dirname", dirname)
			os.Exit(1)
		}
	}

	// If a frontend directory is provided, serve carta_frontend from there
	if cfg.FrontendDir != "" {
		info, err := os.Stat(cfg.FrontendDir)
		if err != nil || !info.IsDir() {
			slog.Error("Failed to set frontendDir", "error", err, "dirname", cfg.FrontendDir)
			os.Exit(1)
		}

		slog.Info("Serving carta_frontend", "dirname", cfg.FrontendDir)
		fs := http.FileServer(http.Dir(cfg.FrontendDir))

		if oidcAuth != nil && (cfg.AuthMode == config.AuthOIDC || cfg.AuthMode == config.AuthBoth) {
			http.Handle("/oidc/login", http.HandlerFunc(oidcAuth.LoginHandler))
			http.Handle("/oidc/callback", http.HandlerFunc(oidcAuth.CallbackHandler))
		}

		// Root handler behaves like carta_backend:
		//  - /           -> index.html
		//  - /static/... -> real files
		//  - /whatever   -> index.html (for SPA routes)
		// Wrap root with the currently-selected authenticator (PAM, OIDC, both, or none).
		http.Handle("/", withAuth(authenticator, spaHandler{
			root: cfg.FrontendDir,
			fs:   fs,
		}))

		// Expose the PAM login page only when PAM is enabled.
		if pamAuth != nil && (cfg.AuthMode == config.AuthPAM || cfg.AuthMode == config.AuthBoth) {
			http.Handle("/pam-login", pamLoginHandler(pamAuth))
		}
	} else {
		slog.Info("No --frontendDir supplied: controller will *not* serve the frontend (only /carta WebSocket).")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Hostname, cfg.Port)

	slog.Info("Server listening", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}
