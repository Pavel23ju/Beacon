// Package testdb starts a throwaway PostGIS container for integration tests.
package testdb

import (
	"context"
	"os"
	"time"

	"github.com/Pavel23ju/beacon/internal/platform"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Start launches PostGIS, applies migrations and returns a pool plus a
// cleanup func. Intended for TestMain, so it panics on failure.
func Start() (*pgxpool.Pool, func()) {
	ctx := context.Background()
	image := os.Getenv("POSTGIS_IMAGE")
	if image == "" {
		image = "imresamu/postgis:17-3.5"
	}
	pg, err := postgres.Run(ctx, image,
		postgres.WithDatabase("beacon"),
		postgres.WithUsername("beacon"),
		postgres.WithPassword("beacon"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(2*time.Minute)),
	)
	if err != nil {
		panic(err)
	}
	url, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(err)
	}
	pool, err := platform.OpenDB(ctx, url)
	if err != nil {
		panic(err)
	}
	return pool, func() {
		pool.Close()
		_ = pg.Terminate(ctx)
	}
}
