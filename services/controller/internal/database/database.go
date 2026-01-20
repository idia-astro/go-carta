package database

import (
	"log/slog"
	"net/http"
    "fmt"
    "os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

    "github.com/santhosh-tekuri/jsonschema/v6"
    "embed"
    "encoding/json"

    "github.com/CARTAvis/go-carta/services/controller/internal/session"
    "github.com/CARTAvis/go-carta/services/controller/internal/auth"

)

// Avoid needing to ship schema files separately
//go:embed schemas/preferences_schema_2.json
//go:embed schemas/layout_schema_2.json
//go:embed schemas/snippet_schema_1.json
//go:embed schemas/workspace_schema_1.json
var schemaFiles embed.FS

const PREFERENCE_SCHEMA_VERSION = 2;
//const LAYOUT_SCHEMA_VERSION = 2;
//const SNIPPET_SCHEMA_VERSION = 1;
//const WORKSPACE_SCHEMA_VERSION = 0;

func loadSchema(c *jsonschema.Compiler, path string) (*jsonschema.Schema, error) {
    f, err := schemaFiles.Open(path)
    if err != nil {
        return nil, err
    }
    defer func () {
        if err := f.Close(); err != nil {
            slog.Error("error closing file %s: %v", path, err)
        }
    }()

    inst, err := jsonschema.UnmarshalJSON(f)
    if err != nil {
	    slog.Error("UnmarshalJSON error: %v", err)
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
		slog.Error("Error connecting to database: %v", err)
        os.Exit(-1)
	}
	h.db = db

	// Ensure tables exist
	if err := h.EnsureTables(); err != nil {
		slog.Error("Error ensuring database tables: %v", err)
        os.Exit(-1)
	}

    // Load JSON schemas
    c := jsonschema.NewCompiler()
    h.PrefSchema, err = loadSchema(c, "schemas/preferences_schema_2.json")
    if err != nil {
        slog.Error("Error loading preferences schema: %v", err)
        os.Exit(-1)
    }
    h.LayoutSchema, err = loadSchema(c, "schemas/layout_schema_2.json")
    if err != nil {
        slog.Error("Error loading layout schema: %v", err)
        os.Exit(-1)
    }
    h.SnippetSchema, err = loadSchema(c, "schemas/snippet_schema_1.json")
    if err != nil {
        slog.Error("Error loading snippet schema: %v", err)
        os.Exit(-1)
    }
    h.WorkspaceSchema, err = loadSchema(c, "schemas/workspace_schema_1.json")
    if err != nil {
        slog.Error("Error loading workspace schema: %v", err)
        os.Exit(-1)
    }
}

func getUsername(r *http.Request) string {
    ctx := r.Context()
    user, ok := ctx.Value(session.UserContextKey).(*auth.User)
    if !ok {
        slog.Error("No username found in request context")
        return ""
    }
    return user.Username
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
    slog.Info("DB API called: %s %s (not implemented)", r.Method, r.URL.Path)
    w.WriteHeader(http.StatusNotImplemented)
    _, _ = w.Write([]byte("Not implemented"))
}

func writeJSONResponse(w http.ResponseWriter, status int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)

    resp := map[string]any{
        "status_code": status,
        "message":     message,
    }

    if err := json.NewEncoder(w).Encode(resp); err != nil {
        slog.Error("Error encoding JSON response: %v", err)
    }
}


func (h *DbConfig) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
    slog.Debug("DB API called: %s %s", r.Method, r.URL.Path)

    empty := map[string]any{}
    empty["version"] = 2
    err := h.PrefSchema.Validate(empty)
    if err != nil {
        http.Error(w, fmt.Sprintf("validation failed: %v", err), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    if err := json.NewEncoder(w).Encode(map[string]any{
        "success": true,
        "preferences": empty,
    }); err != nil {
        slog.Error("Error encoding JSON response: %v", err)
    }
}

func (h *DbConfig) handleSetPreferences(w http.ResponseWriter, r *http.Request) {
    slog.Debug("DB API called: %s %s", r.Method, r.URL.Path)

    user := getUsername(r)
    if user == "" {
        // No username means an error ... rather than unauthorized as `withAuth` should have caught this
        http.Error(w, "Username not found, but passed authorization", http.StatusInternalServerError)
    }

    // Decode JSON body
    var prefs map[string]any
    dec := json.NewDecoder(r.Body)
    if err := dec.Decode(&prefs); err != nil {
        writeJSONResponse(w, http.StatusBadRequest, fmt.Sprintf("Failed to decode JSON body: %v", err))
        return
    }

    // Validate against schema
    prefs["version"] = PREFERENCE_SCHEMA_VERSION
    if err := h.PrefSchema.Validate(prefs); err != nil {
        writeJSONResponse(w, http.StatusBadRequest, fmt.Sprintf("Preferences validation failed: %v", err))
        return
    }

    // Persist to DB
    _, err := h.db.ExecContext(r.Context(),
        `INSERT INTO preferences (username, content)
        VALUES ($1, $2)
        ON CONFLICT (username)
        DO UPDATE SET content = preferences.content || EXCLUDED.content`,
        user, prefs,
    )
    if err != nil {
        writeJSONResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to store preferences: %v", err))
        return
    }

    w.Header().Set("Content-Type", "application/json")
    writeJSONResponse(w, http.StatusOK, "Preferences set successfully")
}

func (h *DbConfig) Router() http.Handler {
    mux := http.NewServeMux()

    mux.Handle("GET /preferences", http.HandlerFunc(h.handleGetPreferences))
    mux.Handle("PUT /preferences", http.HandlerFunc(h.handleSetPreferences))
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
    slog.Info("Received request for database handler: %s %s", r.Method, r.URL.Path)

    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte("hello from the database world"))
}