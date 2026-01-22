package database

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"database/sql"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"embed"
	"encoding/json"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/CARTAvis/go-carta/services/controller/internal/auth"
	"github.com/CARTAvis/go-carta/services/controller/internal/session"
)

// Avoid needing to ship schema files separately
//
//go:embed schemas/preferences_schema_2.json
//go:embed schemas/layout_schema_2.json
//go:embed schemas/snippet_schema_1.json
//go:embed schemas/workspace_schema_1.json
var schemaFiles embed.FS

const PREFERENCE_SCHEMA_VERSION = 2
const LAYOUT_SCHEMA_VERSION = 2
const SNIPPET_SCHEMA_VERSION = 1

//const WORKSPACE_SCHEMA_VERSION = 0;

func loadSchema(c *jsonschema.Compiler, path string) (*jsonschema.Schema, error) {
	f, err := schemaFiles.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("error closing file %s: %v", path, err)
		}
	}()

	inst, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		slog.Error("UnmarshalJSON error", "err", err)
	}
	if err := c.AddResource("embed://"+path, inst); err != nil {
		return nil, err
	}
	return c.Compile("embed://" + path)
}

type DbConfig struct {
	ConnString string
	db         *sqlx.DB

	// Compiled schemas
	PrefSchema      *jsonschema.Schema
	LayoutSchema    *jsonschema.Schema
	WorkspaceSchema *jsonschema.Schema
	SnippetSchema   *jsonschema.Schema
}

func (h *DbConfig) EnsureTables() error {
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
		slog.Error("Error connecting to database", "err", err)
		os.Exit(-1)
	}
	h.db = db

	// Ensure tables exist
	if err := h.EnsureTables(); err != nil {
		slog.Error("Error ensuring database tables", "err", err)
		os.Exit(-1)
	}

	// Load JSON schemas
	c := jsonschema.NewCompiler()
	h.PrefSchema, err = loadSchema(c, "schemas/preferences_schema_2.json")
	if err != nil {
		slog.Error("Error loading preferences schema", "err", err)
		os.Exit(-1)
	}
	h.LayoutSchema, err = loadSchema(c, "schemas/layout_schema_2.json")
	if err != nil {
		slog.Error("Error loading layout schema", "err", err)
		os.Exit(-1)
	}
	h.SnippetSchema, err = loadSchema(c, "schemas/snippet_schema_1.json")
	if err != nil {
		slog.Error("Error loading snippet schema", "err", err)
		os.Exit(-1)
	}
	h.WorkspaceSchema, err = loadSchema(c, "schemas/workspace_schema_1.json")
	if err != nil {
		slog.Error("Error loading workspace schema", "err", err)
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
	slog.Warn(fmt.Sprintf("DB API called: %s %s (not implemented)", r.Method, r.URL.Path))
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
		slog.Error("Error encoding JSON response", "err", err)
	}
}

func (h *DbConfig) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	slog.Debug(fmt.Sprintf("DB API called: %s %s", r.Method, r.URL.Path))

	user := getUsername(r)
	if user == "" {
		// No username means an error ... rather than unauthorized as `withAuth` should have caught this
		writeJSONResponse(w, http.StatusInternalServerError, "Username not found, but passed authorization")
		return
	}

	// Query DB
	var raw json.RawMessage
	err := h.db.GetContext(r.Context(), &raw,
		`SELECT content FROM preferences WHERE username = $1`,
		user,
	)
	var prefs map[string]any

	switch {
	case err == sql.ErrNoRows:
		// Defaults
		prefs = map[string]any{"version": PREFERENCE_SCHEMA_VERSION}
		if err := h.PrefSchema.Validate(prefs); err != nil {
			writeJSONResponse(w, http.StatusInternalServerError, fmt.Sprintf("Default preferences validation failed: %v", err))
			return
		}

	case err != nil:
		writeJSONResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to query preferences: %v", err))
		return

	default:
		// Decode JSONB from DB
		if err := json.Unmarshal(raw, &prefs); err != nil {
			writeJSONResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to decode stored preferences: %v", err))
			return
		}

		// Validate + apply defaults
		if err := h.PrefSchema.Validate(prefs); err != nil {
			writeJSONResponse(w, http.StatusInternalServerError, fmt.Sprintf("Stored preferences failed validation: %v", err))
			return
		}
	}

	// Respond
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"success":     true,
		"preferences": prefs,
	}); err != nil {
		slog.Error("Error encoding JSON response", "err", err)
	}
}

func (h *DbConfig) handleSetPreferences(w http.ResponseWriter, r *http.Request) {
	slog.Debug(fmt.Sprintf("DB API called: %s %s", r.Method, r.URL.Path))

	user := getUsername(r)
	if user == "" {
		// No username means an error ... rather than unauthorized as `withAuth` should have caught this
		writeJSONResponse(w, http.StatusInternalServerError, "Username not found, but passed authorization")
		return
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

	// Remarshal the preferences to ensure proper JSONB format
	jsonBytes, err := json.Marshal(prefs)
	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, "Failed to encode preferences as JSON")
		return
	}

	// Persist to DB
	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO preferences (username, content)
        VALUES ($1, $2::jsonb)
        ON CONFLICT (username)
        DO UPDATE SET content = preferences.content || EXCLUDED.content`,
		user, jsonBytes,
	)
	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to store preferences: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSONResponse(w, http.StatusOK, "Preferences set successfully")
}

func (h *DbConfig) handleClearPreferences(w http.ResponseWriter, r *http.Request) {
	slog.Debug(fmt.Sprintf("DB API called: %s %s", r.Method, r.URL.Path))

	user := getUsername(r)
	if user == "" {
		writeJSONResponse(w, http.StatusInternalServerError, "Username not found, but passed authorization")
		return
	} else {
		slog.Debug("Clearing preferences for user", "user", user)
	}

	// Parse JSON body
	var body struct {
		Keys []string `json:"keys"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		writeJSONResponse(w, http.StatusBadRequest, "Malformed JSON body")
		return
	}
	// Validate keys list
	if len(body.Keys) == 0 {
		slog.Debug("Malformed key list received for clearing preferences")
		writeJSONResponse(w, http.StatusBadRequest, "Malformed key list")
		return
	}

	slog.Debug("Clearing preference keys", "user", user, "keys", body.Keys)

	// Update DB to remove keys
	_, err := h.db.ExecContext(
		r.Context(),
		`UPDATE preferences
         SET content = content - $2::text[]
         WHERE username = $1`,
		user,
		pq.Array(body.Keys),
	)

	if err != nil {
		slog.Error("Error clearing preferences", "err", err)
		writeJSONResponse(w, http.StatusInternalServerError, "Problem clearing preferences")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	}); err != nil {
		slog.Error("Error encoding JSON response", "err", err)
	}
}

func (h *DbConfig) handleGetLayouts(w http.ResponseWriter, r *http.Request) {
	slog.Debug(fmt.Sprintf("DB API called: %s %s", r.Method, r.URL.Path))

	user := getUsername(r)
	if user == "" {
		writeJSONResponse(w, http.StatusInternalServerError, "Username not found, but passed authorization")
		return
	} else {
		slog.Debug("Getting layouts for user", "user", user)
	}

	// Query all layouts for this user
	rows, err := h.db.QueryContext(
		r.Context(),
		`SELECT name, content
         FROM layouts
         WHERE username = $1`,
		user,
	)
	if err != nil {
		slog.Error("DB error fetching layouts", "err", err)
		writeJSONResponse(w, http.StatusInternalServerError, "Failed to fetch layouts")
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Warn("error closing rows", "err", err)
		}
	}()

	// Build the response map
	layouts := make(map[string]any)

	for rows.Next() {
		var name string
		var raw json.RawMessage

		if err := rows.Scan(&name, &raw); err != nil {
			slog.Error("Error scanning layout row", "err", err)
			continue
		}

		// Decode JSONB content
		var layout any
		if err := json.Unmarshal(raw, &layout); err != nil {
			slog.Warn("Invalid JSON in stored layout", "name", name, "err", err)
			continue
		}

		// Validate layout
		if err := h.LayoutSchema.Validate(layout); err != nil {
			slog.Warn("Returning invalid layout", "name", name, "err", err)
		}

		layouts[name] = layout
	}

	if err := rows.Err(); err != nil {
		slog.Error("Row iteration error", "err", err)
		writeJSONResponse(w, http.StatusInternalServerError, "Failed to read layouts")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"layouts": layouts,
	}); err != nil {
		slog.Error("Error encoding JSON response", "err", err)
	}
}

func (h *DbConfig) handleSetLayout(w http.ResponseWriter, r *http.Request) {
	slog.Debug(fmt.Sprintf("DB API called: %s %s", r.Method, r.URL.Path))

	user := getUsername(r)
	if user == "" {
		writeJSONResponse(w, http.StatusInternalServerError, "Username not found, but passed authorization")
		return
	} else {
		slog.Debug("Setting layout for user", "user", user)
	}

	// Parse JSON body
	var body struct {
		Name   string         `json:"layoutName"`
		Layout map[string]any `json:"layout"`
	}

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		writeJSONResponse(w, http.StatusBadRequest, "Malformed JSON body")
		return
	}

	version, ok := body.Layout["layoutVersion"].(float64)
	if body.Name == "" || body.Layout == nil || !ok || int(version) != LAYOUT_SCHEMA_VERSION {
		writeJSONResponse(w, http.StatusBadRequest, "Malformed layout update")
		return
	}

	// Validate layout
	if err := h.LayoutSchema.Validate(body.Layout); err != nil {
		slog.Warn("Layout validation failed", "name", body.Name, "err", err)
		writeJSONResponse(w, http.StatusBadRequest, fmt.Sprintf("Invalid layout update: %v", err))
		return
	}

	// Marshal layout to JSONB
	jsonBytes, err := json.Marshal(body.Layout)
	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, "Failed to encode layout as JSON")
		return
	}

	// UPSERT into Postgres
	_, err = h.db.ExecContext(
		r.Context(),
		`INSERT INTO layouts (name, username, content)
         VALUES ($1, $2, $3::jsonb)
         ON CONFLICT (name, username)
         DO UPDATE SET content = EXCLUDED.content`,
		body.Name, user, jsonBytes,
	)

	if err != nil {
		slog.Error("Failed to store layout", "err", err)
		writeJSONResponse(w, http.StatusInternalServerError, "Failed to store layout")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	}); err != nil {
		slog.Error("Error encoding JSON response", "err", err)
	}
}

func (h *DbConfig) handleClearLayout(w http.ResponseWriter, r *http.Request) {
	slog.Debug(fmt.Sprintf("DB API called: %s %s", r.Method, r.URL.Path))

	user := getUsername(r)
	if user == "" {
		writeJSONResponse(w, http.StatusInternalServerError, "Username not found, but passed authorization")
		return
	} else {
		slog.Debug("Clearing layout for user", "user", user)
	}

	// Parse JSON body
	var body struct {
		LayoutName string `json:"layoutName"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		writeJSONResponse(w, http.StatusBadRequest, "Malformed JSON body")
		return
	}
	// Validate layout name
	if body.LayoutName == "" {
		slog.Debug("Malformed layout name received for clearing layout")
		writeJSONResponse(w, http.StatusBadRequest, "Malformed layout name")
		return
	}

	slog.Debug("Clearing layout", "user", user, "layoutName", body.LayoutName)

	// Update DB to remove layout
	_, err := h.db.ExecContext(
		r.Context(),
		`DELETE FROM layouts
         WHERE name = $2 AND username = $1`,
		user,
		body.LayoutName,
	)

	if err != nil {
		slog.Error("Error removing layout", "err", err)
		writeJSONResponse(w, http.StatusInternalServerError, "Problem removing layout")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	}); err != nil {
		slog.Error("Error encoding JSON response", "err", err)
	}
}

func (h *DbConfig) handleGetSnippets(w http.ResponseWriter, r *http.Request) {
	slog.Debug(fmt.Sprintf("DB API called: %s %s", r.Method, r.URL.Path))

	user := getUsername(r)
	if user == "" {
		writeJSONResponse(w, http.StatusInternalServerError, "Username not found, but passed authorization")
		return
	} else {
		slog.Debug("Getting snippets for user", "user", user)
	}

	// Query all snippets for this user
	rows, err := h.db.QueryContext(
		r.Context(),
		`SELECT name, content
         FROM snippets
         WHERE username = $1`,
		user,
	)
	if err != nil {
		slog.Error("DB error fetching snippets", "err", err)
		writeJSONResponse(w, http.StatusInternalServerError, "Failed to fetch snippets")
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Warn("error closing rows", "err", err)
		}
	}()

	// Build the response map
	snippets := make(map[string]any)

	for rows.Next() {
		var name string
		var raw json.RawMessage

		if err := rows.Scan(&name, &raw); err != nil {
			slog.Error("Error scanning snippet row", "err", err)
			continue
		}

		// Decode JSONB content
		var snippet any
		if err := json.Unmarshal(raw, &snippet); err != nil {
			slog.Warn("Invalid JSON in stored snippet", "name", name, "err", err)
			continue
		}

		// Validate snippet
		if err := h.SnippetSchema.Validate(snippet); err != nil {
			slog.Warn("Returning invalid snippet", "name", name, "err", err)
		}

		snippets[name] = snippet
	}

	if err := rows.Err(); err != nil {
		slog.Error("Row iteration error", "err", err)
		writeJSONResponse(w, http.StatusInternalServerError, "Failed to read snippets")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"snippets": snippets,
	}); err != nil {
		slog.Error("Error encoding JSON response", "err", err)
	}
}

func (h *DbConfig) handleSetSnippet(w http.ResponseWriter, r *http.Request) {
	slog.Debug(fmt.Sprintf("DB API called: %s %s", r.Method, r.URL.Path))

	user := getUsername(r)
	if user == "" {
		writeJSONResponse(w, http.StatusInternalServerError, "Username not found, but passed authorization")
		return
	} else {
		slog.Debug("Setting snippet for user", "user", user)
	}

	// Parse JSON body
	var body struct {
		Name    string         `json:"snippetName"`
		Snippet map[string]any `json:"snippet"`
	}

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		writeJSONResponse(w, http.StatusBadRequest, "Malformed JSON body")
		return
	}

	version, ok := body.Snippet["snippetVersion"].(float64)
	if body.Name == "" || body.Snippet == nil || !ok || int(version) != SNIPPET_SCHEMA_VERSION {
		writeJSONResponse(w, http.StatusBadRequest, "Malformed snippet update")
		return
	}

	// Validate snippet
	if err := h.SnippetSchema.Validate(body.Snippet); err != nil {
		slog.Warn("Snippet validation failed", "name", body.Name, "err", err)
		writeJSONResponse(w, http.StatusBadRequest, fmt.Sprintf("Invalid snippet update: %v", err))
		return
	}

	// Marshal snippet to JSONB
	jsonBytes, err := json.Marshal(body.Snippet)
	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, "Failed to encode snippet as JSON")
		return
	}

	// UPSERT into Postgres
	_, err = h.db.ExecContext(
		r.Context(),
		`INSERT INTO snippets (name, username, content)
         VALUES ($1, $2, $3::jsonb)
         ON CONFLICT (name, username)
         DO UPDATE SET content = EXCLUDED.content`,
		body.Name, user, jsonBytes,
	)

	if err != nil {
		slog.Error("Failed to store snippet", "err", err)
		writeJSONResponse(w, http.StatusInternalServerError, "Failed to store snippet")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	}); err != nil {
		slog.Error("Error encoding JSON response", "err", err)
	}
}

func (h *DbConfig) handleClearSnippet(w http.ResponseWriter, r *http.Request) {
	slog.Debug(fmt.Sprintf("DB API called: %s %s", r.Method, r.URL.Path))

	user := getUsername(r)
	if user == "" {
		writeJSONResponse(w, http.StatusInternalServerError, "Username not found, but passed authorization")
		return
	} else {
		slog.Debug("Clearing snippet for user", "user", user)
	}

	// Parse JSON body
	var body struct {
		SnippetName string `json:"snippetName"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		writeJSONResponse(w, http.StatusBadRequest, "Malformed JSON body")
		return
	}
	// Validate snippet name
	if body.SnippetName == "" {
		slog.Debug("Malformed snippet name received for clearing snippet")
		writeJSONResponse(w, http.StatusBadRequest, "Malformed snippet name")
		return
	}

	slog.Debug("Clearing snippet", "user", user, "snippetName", body.SnippetName)

	// Update DB to remove snippet
	_, err := h.db.ExecContext(
		r.Context(),
		`DELETE FROM snippets
         WHERE name = $2 AND username = $1`,
		user,
		body.SnippetName,
	)

	if err != nil {
		slog.Error("Error removing snippet", "err", err)
		writeJSONResponse(w, http.StatusInternalServerError, "Problem removing snippet")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	}); err != nil {
		slog.Error("Error encoding JSON response", "err", err)
	}
}

func (h *DbConfig) Router() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /preferences", http.HandlerFunc(h.handleGetPreferences))
	mux.Handle("PUT /preferences", http.HandlerFunc(h.handleSetPreferences))
	mux.Handle("DELETE /preferences", http.HandlerFunc(h.handleClearPreferences))

	mux.Handle("GET /layouts", http.HandlerFunc(h.handleGetLayouts))
	mux.Handle("PUT /layout", http.HandlerFunc(h.handleSetLayout))
	mux.Handle("DELETE /layout", http.HandlerFunc(h.handleClearLayout))

	mux.Handle("GET /snippets", http.HandlerFunc(h.handleGetSnippets))
	mux.Handle("PUT /snippet", http.HandlerFunc(h.handleSetSnippet))
	mux.Handle("DELETE /snippet", http.HandlerFunc(h.handleClearSnippet))

	mux.Handle("POST /share/workspace/{id}", http.HandlerFunc(notImplemented))

	mux.Handle("GET /list/workspaces", http.HandlerFunc(notImplemented))
	mux.Handle("GET /workspace/key/{key}", http.HandlerFunc(notImplemented))
	mux.Handle("GET /workspace/{name}", http.HandlerFunc(notImplemented))
	mux.Handle("PUT /workspace", http.HandlerFunc(notImplemented))
	mux.Handle("DELETE /workspace", http.HandlerFunc(notImplemented))

	return mux
}
