package incident

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pavel23ju/beacon/internal/eventstore"
	"github.com/Pavel23ju/beacon/pkg/events"
	"github.com/Pavel23ju/beacon/pkg/hlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists incidents and their events in Postgres.
type Store struct {
	pool  *pgxpool.Pool
	clock *hlc.Clock
	node  string // origin_node stamped on events written by this process
}

func NewStore(pool *pgxpool.Pool, clock *hlc.Clock, node string) *Store {
	return &Store{pool: pool, clock: clock, node: node}
}

const selectIncident = `
SELECT id, type, priority, status,
       ST_Y(location::geometry), ST_X(location::geometry),
       people_affected, description, reported_at, updated_hlc
FROM incidents`

func scanIncident(row pgx.Row) (Incident, error) {
	var inc Incident
	err := row.Scan(&inc.ID, &inc.Type, &inc.Priority, &inc.Status,
		&inc.Lat, &inc.Lon, &inc.PeopleAffected, &inc.Description,
		&inc.ReportedAt, &inc.UpdatedHLC)
	if errors.Is(err, pgx.ErrNoRows) {
		return Incident{}, ErrNotFound
	}
	return inc, err
}

// Create reports a new incident in REPORTED state.
func (s *Store) Create(ctx context.Context, in NewIncident) (Incident, error) {
	if err := in.Validate(); err != nil {
		return Incident{}, err
	}
	id := uuid.New()
	ts := s.clock.Now()
	reportedAt := time.Now().UTC().Truncate(time.Microsecond) // Postgres precision

	var inc Incident
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO incidents (id, type, status, location, people_affected, description, reported_at, updated_hlc)
			VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, $6, $7, $8, $9)`,
			id, in.Type, Reported, in.Lon, in.Lat, in.PeopleAffected, in.Description, reportedAt, int64(ts))
		if err != nil {
			return fmt.Errorf("insert incident: %w", err)
		}
		payload := reportedPayload{NewIncident: in, ReportedAt: reportedAt}
		if err := eventstore.Append(ctx, tx, id, events.IncidentReported, payload, ts, s.node); err != nil {
			return err
		}
		inc, err = scanIncident(tx.QueryRow(ctx, selectIncident+` WHERE id = $1`, id))
		return err
	})
	return inc, err
}

// Get returns the current state of an incident.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Incident, error) {
	return scanIncident(s.pool.QueryRow(ctx, selectIncident+` WHERE id = $1`, id))
}

// Transition moves an incident to a new status. The row is locked for the
// duration of the transaction, so concurrent transitions are serialized and
// each one is validated against the true current state.
func (s *Store) Transition(ctx context.Context, id uuid.UUID, to Status, priority int16) (Incident, error) {
	if err := ValidatePriority(to, priority); err != nil {
		return Incident{}, err
	}

	var inc Incident
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var from Status
		err := tx.QueryRow(ctx, `SELECT status FROM incidents WHERE id = $1 FOR UPDATE`, id).Scan(&from)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := CanTransition(from, to); err != nil {
			return err
		}

		ts := s.clock.Now()
		_, err = tx.Exec(ctx, `
			UPDATE incidents
			SET status = $2, updated_hlc = $3,
			    priority = CASE WHEN $4::smallint > 0 THEN $4::smallint ELSE priority END
			WHERE id = $1`,
			id, to, int64(ts), priority)
		if err != nil {
			return fmt.Errorf("update incident: %w", err)
		}
		payload := transitionedPayload{From: from, To: to, Priority: priority}
		if err := eventstore.Append(ctx, tx, id, events.IncidentTransitioned, payload, ts, s.node); err != nil {
			return err
		}
		inc, err = scanIncident(tx.QueryRow(ctx, selectIncident+` WHERE id = $1`, id))
		return err
	})
	return inc, err
}

// Events returns the event history of an incident in HLC order.
func (s *Store) Events(ctx context.Context, id uuid.UUID) ([]events.Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT event_id, aggregate_id, type, payload, hlc, origin_node, received_at
		FROM events WHERE aggregate_id = $1 ORDER BY hlc, event_id`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (events.Event, error) {
		var ev events.Event
		err := row.Scan(&ev.EventID, &ev.AggregateID, &ev.Type, &ev.Payload,
			&ev.HLC, &ev.OriginNode, &ev.ReceivedAt)
		return ev, err
	})
}
