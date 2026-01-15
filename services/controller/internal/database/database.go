package database

import (
	"log"
	"net/http"
    "fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

    "github.com/santhosh-tekuri/jsonschema/v6"
    "embed"
    "encoding/json"

    "github.com/idia-astro/go-carta/services/controller/internal/session"
    "github.com/idia-astro/go-carta/services/controller/internal/auth"

)

// Avoid needing to ship schema files separately
//go:embed schemas/preferences_schema_2.json
//go:embed schemas/layout_schema_2.json
//go:embed schemas/snippet_schema_1.json
//go:embed schemas/workspace_schema_1.json
var schemaFiles embed.FS

func loadSchema(c *jsonschema.Compiler, path string) (*jsonschema.Schema, error) {
    f, err := schemaFiles.Open(path)
    if err != nil {
        return nil, err
    }
    defer func () {
        if err := f.Close(); err != nil {
            log.Printf("error closing file %s: %v", path, err)
        }
    }()

    inst, err := jsonschema.UnmarshalJSON(f)
    if err != nil {
	    log.Fatal(err)
    }
    if err := c.AddResource("embed://"+path, inst); err != nil {
        return nil, err
    }
    return c.Compile("embed://"+path)
}

type DbConfig struct {
	ConnString string
	db         *sqlx.DB

    // Compiled schemas
    PrefSchema     *jsonschema.Schema
    LayoutSchema   *jsonschema.Schema
    WorkspaceSchema *jsonschema.Schema
    SnippetSchema  *jsonschema.Schema
}

func (h *DbConfig) EnsureTables() error{
	schema := `
    CREATE TABLE IF NOT EXISTS preferences (
        username TEXT PRIMARY KEY,
        content  JSONB NOT NULL
    );

    CREATE TABLE IF NOT EXISTS layouts (
        name     TEXT NOT NULL,
        username TEXT NOT NULL,
        content  JSONB NOT NULL,
        PRIMARY KEY (name, username)
    );

    CREATE TABLE IF NOT EXISTS snippets (
        name     TEXT NOT NULL,
        username TEXT NOT NULL,
        content  JSONB NOT NULL,
        PRIMARY KEY (name, username)
    );

    CREATE TABLE IF NOT EXISTS workspaces (
        name     TEXT NOT NULL,
        username TEXT NOT NULL,
        content  JSONB NOT NULL,
        PRIMARY KEY (name, username)
    );
    `

    if _, err := h.db.Exec(schema); err != nil {
        return err
    }
    return nil
}

func (h *DbConfig) InitDb() {
	// Initialize DB connection
	db, err := sqlx.Connect("postgres", h.ConnString)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	h.db = db

	// Ensure tables exist
	if err := h.EnsureTables(); err != nil {
		log.Fatalf("Error ensuring database tables: %v", err)
	}

    // Load JSON schemas
    c := jsonschema.NewCompiler()
    h.PrefSchema, err = loadSchema(c, "schemas/preferences_schema_2.json")
    if err != nil {
        log.Fatal(err)
    }
    h.LayoutSchema, err = loadSchema(c, "schemas/layout_schema_2.json")
    if err != nil {
        log.Fatal(err)
    }
    h.SnippetSchema, err = loadSchema(c, "schemas/snippet_schema_1.json")
    if err != nil {
        log.Fatal(err)
    }
    h.WorkspaceSchema, err = loadSchema(c, "schemas/workspace_schema_1.json")
    if err != nil {
        log.Fatal(err)
    }
}

func getUsername(r *http.Request) string {
    ctx := r.Context()
    user, ok := ctx.Value(session.UserContextKey).(*auth.User)
    if !ok {
        log.Printf("No username found in request context")
        return ""
    }
    return user.Username
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
    log.Printf("DB API called: %s %s (not implemented)", r.Method, r.URL.Path)
    w.WriteHeader(http.StatusNotImplemented)
    _, _ = w.Write([]byte("Not implemented"))
}

func (h *DbConfig) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
    //log.Printf("get username: %s", getUsername(r))
    empty := map[string]any{}
    empty["version"] = 2
    err := h.PrefSchema.Validate(empty)
    if err != nil {
        http.Error(w, fmt.Sprintf("validation failed: %v", err), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)

    enc := json.NewEncoder(w)
    enc.Encode(empty)
}


func (h *DbConfig) Router() http.Handler {
    mux := http.NewServeMux()

    mux.Handle("GET /preferences", http.HandlerFunc(h.handleGetPreferences))
    mux.Handle("PUT /preferences", http.HandlerFunc(notImplemented))
    mux.Handle("DELETE /preferences", http.HandlerFunc(notImplemented))

    mux.Handle("GET /layouts", http.HandlerFunc(notImplemented))
    mux.Handle("PUT /layout", http.HandlerFunc(notImplemented))
    mux.Handle("DELETE /layout", http.HandlerFunc(notImplemented))

    mux.Handle("GET /snippets", http.HandlerFunc(notImplemented))
    mux.Handle("PUT /snippet", http.HandlerFunc(notImplemented))
    mux.Handle("DELETE /snippet", http.HandlerFunc(notImplemented))

    mux.Handle("POST /share/workspace/{id}", http.HandlerFunc(notImplemented))

    mux.Handle("GET /list/workspaces", http.HandlerFunc(notImplemented))
    mux.Handle("GET /workspace/key/{key}", http.HandlerFunc(notImplemented))
    mux.Handle("GET /workspace/{name}", http.HandlerFunc(notImplemented))
    mux.Handle("PUT /workspace", http.HandlerFunc(notImplemented))
    mux.Handle("DELETE /workspace", http.HandlerFunc(notImplemented))

    mux.Handle("/", http.HandlerFunc(h.HttpHandler));

    return mux
}



func (h *DbConfig) HttpHandler(w http.ResponseWriter, r *http.Request) {
    log.Printf("Received request for database handler: %s %s", r.Method, r.URL.Path)

    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte("hello from the database world"))
}