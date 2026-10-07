// Package responder manages the registry of field units (ambulance, fire,
// rescue). Live locations arrive via the location service in week 2; this
// package owns identity, capabilities and availability.
package responder

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Pavel23ju/beacon/internal/eventstore"
	"github.com/Pavel23ju/beacon/pkg/events"
	"github.com/Pavel23ju/beacon/pkg/hlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Unit types.
const (
	Ambulance = "ambulance"
	Fire      = "fire"
	Rescue    = "rescue"
)

var validTypes = []string{Ambulance, Fire, Rescue}

// Availability statuses.
const (
	Available = "AVAILABLE"
	Busy      = "BUSY"
	Offline   = "OFFLINE"
)

// Responder is a field unit.
type Responder struct {
	ID           uuid.UUID  `json:"id"`
	Type         string     `json:"type"`
	Capabilities []string   `json:"capabilities"`
	Status       string     `json:"status"`
	Lat          *float64   `json:"lat"` // nil until the first location ping
	Lon          *float64   `json:"lon"`
	LastSeen     *time.Time `json:"last_seen"`
	Mode         string     `json:"mode"` // online | mesh
}

// NewResponder is the input for registering a unit.
type NewResponder struct {
	Type         string   `json:"type"`
	Capabilities []string `json:"capabilities"`
}

var (
	ErrInvalid  = errors.New("invalid input")
	ErrNotFound = errors.New("responder not found")
)

func (n NewResponder) Validate() error {
	if !slices.Contains(validTypes, n.Type) {
		return fmt.Errorf("%w: type must be one of %v", ErrInvalid, validTypes)
	}
	for _, c := range n.Capabilities {
		if c == "" {
			return fmt.Errorf("%w: empty capability", ErrInvalid)
		}
	}
	return nil
}

// Store persists responders in Postgres.
type Store struct {
	pool  *pgxpool.Pool
	clock *hlc.Clock
	node  string
}

func NewStore(pool *pgxpool.Pool, clock *hlc.Clock, node string) *Store {
	return &Store{pool: pool, clock: clock, node: node}
}

const selectResponder = `
SELECT id, type, capabilities, status,
       ST_Y(last_location::geometry), ST_X(last_location::geometry),
       last_seen, mode
FROM responders`

func scanResponder(row pgx.Row) (Responder, error) {
	var r Responder
	err := row.Scan(&r.ID, &r.Type, &r.Capabilities, &r.Status, &r.Lat, &r.Lon, &r.LastSeen, &r.Mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return Responder{}, ErrNotFound
	}
	return r, err
}

// Register adds a unit in AVAILABLE status.
func (s *Store) Register(ctx context.Context, in NewResponder) (Responder, error) {
	if err := in.Validate(); err != nil {
		return Responder{}, err
	}
	caps := slices.Compact(slices.Sorted(slices.Values(in.Capabilities)))
	if caps == nil {
		caps = []string{}
	}
	id := uuid.New()

	var r Responder
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO responders (id, type, capabilities, status) VALUES ($1, $2, $3, $4)`,
			id, in.Type, caps, Available)
		if err != nil {
			return fmt.Errorf("insert responder: %w", err)
		}
		payload := NewResponder{Type: in.Type, Capabilities: caps}
		if err := eventstore.Append(ctx, tx, id, events.ResponderRegistered, payload, s.clock.Now(), s.node); err != nil {
			return err
		}
		r, err = scanResponder(tx.QueryRow(ctx, selectResponder+` WHERE id = $1`, id))
		return err
	})
	return r, err
}

// Get returns one responder.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Responder, error) {
	return scanResponder(s.pool.QueryRow(ctx, selectResponder+` WHERE id = $1`, id))
}

// List returns responders, optionally filtered by status, ordered by id.
func (s *Store) List(ctx context.Context, status string) ([]Responder, error) {
	rows, err := s.pool.Query(ctx, selectResponder+`
		WHERE $1 = '' OR status = $1 ORDER BY id`, status)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Responder, error) {
		return scanResponder(row)
	})
}
