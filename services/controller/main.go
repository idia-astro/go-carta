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

	"idia-astro/go-carta/services/controller/internal/config"
	"idia-astro/go-carta/services/controller/internal/session"

	"idia-astro/go-carta/services/controller/internal/auth"
	authoidc "idia-astro/go-carta/services/controller/internal/auth/oidc"

	authpam "idia-astro/go-carta/services/controller/internal/auth/pam"
	pamauth "idia-astro/go-carta/services/controller/internal/auth/pam"
)

/*
	XXX

var (

	port           = flag.Int("port", 8081, "TCP server port")
	hostname       = flag.String("hostname", "", "Hostname to listen on")
	spawnerAddress = flag.String("spawnerAddress", "http://localhost:8080", "Address of the process spawner")
	baseFolder     = flag.String("baseFolder", "", "Base folder to use")

	// NEW: where the built carta_frontend (index.html, static/, etc.) lives
	frontendDir = flag.String("frontendDir", "", "Path to built carta_frontend assets (e.g. /path/to/carta_frontend/build)")

)
*/
var (
	runtimeSpawnerAddress string
	runtimeBaseFolder     string
	pamAuth               *pamauth.PAMAuthenticator
)

var upgrader = websocket.Upgrader{
	// Ignore Origin header
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// spaHandler serves static files if they exist, otherwise falls back to index.html
type spaHandler struct {
	root string
	fs   http.Handler
}

type rootHandler struct {
	spa http.Handler // your spaHandler
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If this is a WebSocket upgrade (e.g. ws://localhost:8081), hand it to wsHandler
	if websocket.IsWebSocketUpgrade(r) {
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
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	log.Print("Client connected")
	//	defer shared.CloseOrLog(c) // assuming this is the helper you have

	user, _ := r.Context().Value(session.UserContextKey).(*auth.User)
	// upgrade to WebSocket etc.
	//	c, err := upgrader.Upgrade(w, r, nil)
	//    XXX  if err != nil { ... }

	s := session.NewSession(c, runtimeSpawnerAddress, runtimeBaseFolder, user)

	/* XXX	s := session.Session{
		SpawnerAddress: *spawnerAddress,
		BaseFolder:     *baseFolder,
		WebSocket:      c,
	}
	*/
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
			// Authenticator can already have written error; just stop here.
			log.Printf("Auth failed: %v", err)
			return
		}

		// Attach user to context
		ctx := context.WithValue(r.Context(), session.UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func pamLoginHandler(p *pamauth.PAMAuthenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `
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

			if err := pamauth.SetPAMSessionCookie(w, user.Username); err != nil {
				log.Printf("Failed to set PAM session cookie: %v", err)
				http.Error(w, "Session error", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/", http.StatusFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

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
		// XXX	authenticator = authpam.New(cfg.PAM)
		p := pamauth.New(cfg.PAM)
		pamAuth = p
		authenticator = p
	case config.AuthOIDC:
		authenticator = authoidc.New(cfg.OIDC)
	case config.AuthBoth:
		authenticator = auth.Multi(
			authpam.New(cfg.PAM),
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

	// WebSocket endpoint (same as before)
	// XXX
	//	http.HandleFunc("/carta", wsHandler)
	//	http.Handle("/carta", withAuth(authenticator, http.HandlerFunc(wsHandler)))

	// If a frontend directory is provided, serve carta_frontend from there
	if cfg.FrontendDir != "" {
		info, err := os.Stat(cfg.FrontendDir)
		if err != nil || !info.IsDir() {
			log.Fatalf("Invalid --frontendDir %q: %v\n", cfg.FrontendDir, err)
		}

		log.Printf("Serving carta_frontend from %s\n", cfg.FrontendDir)
		fs := http.FileServer(http.Dir(cfg.FrontendDir))

		// Root handler behaves like carta_backend:
		//  - /           -> index.html
		//  - /static/... -> real files
		//  - /whatever   -> index.html (for SPA routes)
		http.Handle("/", withAuth(authenticator, spaHandler{
			root: cfg.FrontendDir,
			fs:   fs,
		}))

		// Expose the PAM login page if PAM is enabled.
		if pamAuth != nil {
			http.Handle("/pam-login", pamLoginHandler(pamAuth))
		}
	} else {
		log.Print("No --frontendDir supplied: controller will *not* serve the frontend (only /carta WebSocket).")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Hostname, cfg.Port)
	log.Printf("Listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))

}
