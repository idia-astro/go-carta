package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

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
		log.Printf("Upgrading WebSocket connection from Origin: %s", r.Header.Get("Origin"))
		return true
	},
}

// spaHandler serves static files if they exist, otherwise falls back to index.html
type spaHandler struct {
	root string
	fs   http.Handler
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If this is a WebSocket upgrade (e.g. ws://localhost:8081), hand it to wsHandler
	log.Printf("spaHandler: Received request for %s", r.URL.Path)
	if websocket.IsWebSocketUpgrade(r) {
		log.Printf("** Upgrading to WebSocket for %s", r.URL.Path)
		wsHandler(w, r)
		return
	}

	// Clean and resolve requested path
	path := r.URL.Path
	if path == "" || path == "/" {
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
	log.Printf("** Handling WebSocket connection from %s", r.RemoteAddr)
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}

	defer func() {
		if err := c.Close(); err != nil {
			log.Printf("Error closing WebSocket: %v", err)
		}
	}()

	user, _ := r.Context().Value(session.UserContextKey).(*auth.User)

	s := session.NewSession(c, runtimeSpawnerAddress, runtimeBaseFolder, user)
	log.Printf("Created new session for user: %v", user)
	log.Printf(".   -----   %+v\n", s)

	// Close worker on exit if it exists
	defer s.HandleDisconnect()

	// Basic handler based on gorilla/websocket example
	for {
		messageType, message, err := c.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}

		// Ping/pong sequence
		if messageType == websocket.TextMessage && string(message) == "PING" {
			log.Printf("Received PING from client, sending PONG\n")
			err := c.WriteMessage(websocket.TextMessage, []byte("PONG"))
			if err != nil {
				log.Printf("Failed to send pong message: %v\n", err)
			}
			continue
		}

		// Ignore all other non-binary messages
		if messageType != websocket.BinaryMessage {
			log.Printf("Ignoring non-binary message: %s\n", message)
			continue
		}

		go func() {
			err := s.HandleMessage(message)
			if err != nil {
				log.Printf("Failed to handle message: %v\n", err)
			}
		}()
	}

	// defer should shut down the worker afterwards
	log.Print("Client disconnected")
}

func withAuth(a auth.Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := a.AuthenticateHTTP(w, r)
		if err != nil {
			// Expected when we just redirected to /oidc/login
			if strings.Contains(err.Error(), "no OIDC session") {
				// optional: log at debug level instead
				log.Printf("Auth: redirecting to OIDC login")
				return
			}
			log.Printf("Auth failed: %v", err)
			return
		}

		// Attach user to context
		ctx := context.WithValue(r.Context(), session.UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func pamLoginHandler(p pamwrap.Authenticator) http.Handler {
	log.Printf("Setting up PAM login handler")
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
				log.Printf("PAM login failed for %q: %v", username, err)
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}

			if err := pamwrap.SetSessionCookie(w, user.Username); err != nil {
				log.Printf("Failed to set PAM session cookie: %v", err)
				http.Error(w, "Session error", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/", http.StatusFound)
		default:
			log.Printf("Unsupported PAM login method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

var oidcAuth *authoidc.OIDCAuthenticator

func main() {
	id := uuid.New()
	log.Printf("Starting controller with UUID: %s\n", id.String())

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

	log.Printf("Auth mode: %s", cfg.AuthMode)

	switch cfg.AuthMode {
	case config.AuthNone:
		authenticator = auth.NoopAuthenticator{}
	case config.AuthPAM:
		p, err := pamwrap.New(cfg.PAM)
		if err != nil {
			log.Fatalf("PAM is not available on this platform: %v", err)
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
			log.Fatalf("Auth mode 'both' requires PAM, but PAM is not available on this platform: %v", err)
		}
		pamAuth = p
		authenticator = auth.Multi(
			p,
			authoidc.New(cfg.OIDC),
		)

	default:
		log.Fatalf("Unknown authMode %q", cfg.AuthMode)
	}
	// Default baseFolder to $HOME if unset
	if len(strings.TrimSpace(cfg.BaseFolder)) == 0 {
		dirname, err := os.UserHomeDir()
		if err != nil {
			dirname = "/"
		}
		if err := flag.Set("baseFolder", dirname); err != nil {
			log.Fatalf("Failed to set --baseFolder: %v\n", err)
		}
	}

	// If a frontend directory is provided, serve carta_frontend from there
	if cfg.FrontendDir != "" {
		info, err := os.Stat(cfg.FrontendDir)
		if err != nil || !info.IsDir() {
			log.Fatalf("Invalid --frontendDir %q: %v\n", cfg.FrontendDir, err)
		}

		log.Printf("Serving carta_frontend from %s\n", cfg.FrontendDir)
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
		log.Print("No --frontendDir supplied: controller will *not* serve the frontend (only /carta WebSocket).")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Hostname, cfg.Port)
	log.Printf("Listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
