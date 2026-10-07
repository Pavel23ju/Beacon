// Package eventstore writes to the Postgres events table. Callers append
// inside the same transaction as their state change, so the log and the
// current-state tables can never disagree.
package eventstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Pavel23ju/beacon/pkg/hlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Append records one event. The payload is marshalled to JSON.
func Append(ctx context.Context, tx pgx.Tx, aggregateID uuid.UUID, typ string, payload any, ts hlc.Timestamp, originNode string) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s payload: %w", typ, err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO events (event_id, aggregate_id, type, payload, hlc, origin_node)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), aggregateID, typ, b, int64(ts), originNode)
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}
