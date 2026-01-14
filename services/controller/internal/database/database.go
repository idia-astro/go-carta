package database

import (
	"log"
	"net/http"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type DbConfig struct {
	ConnString string
	db         *sqlx.DB
}

func (h DbConfig) EnsureTables() error{
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

func (h DbConfig) InitDb() {
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
}

func (h DbConfig) HttpHandler(w http.ResponseWriter, r *http.Request) {
    log.Printf("Received request for database handler: %s %s", r.Method, r.URL.Path)

    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte("hello from the database world"))
}
