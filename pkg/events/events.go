// Package events defines the event envelope shared by the backend and
// field agents. Every state change in the system is recorded as an Event.
package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event types.
const (
	IncidentReported     = "incident.reported"
	IncidentTransitioned = "incident.transitioned"
	ResponderRegistered  = "responder.registered"
)

// Event is the unit of the event store. EventID makes it idempotent: the
// same event received twice (e.g. via mesh and via sync) is stored once.
type Event struct {
	EventID     uuid.UUID       `json:"event_id"`
	AggregateID uuid.UUID       `json:"aggregate_id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	HLC         int64           `json:"hlc"`
	OriginNode  string          `json:"origin_node"`
	ReceivedAt  time.Time       `json:"received_at"`
}
