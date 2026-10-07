// Package platform holds process-level plumbing: config, database, and
// migrations.
package platform

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/Pavel23ju/beacon/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Config is read from the environment.
type Config struct {
	DatabaseURL string
	HTTPAddr    string
	NodeID      string // identifies this process as an event origin
}

func LoadConfig() Config {
	return Config{
		DatabaseURL: getenv("DATABASE_URL", "postgres://beacon:beacon@localhost:5432/beacon?sslmode=disable"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		NodeID:      getenv("NODE_ID", "backend-1"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// OpenDB connects to Postgres and applies pending migrations.
func OpenDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	if err := Migrate(pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate applies the embedded migrations.
func Migrate(pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func(db *sql.DB) { _ = db.Close() }(db)

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
