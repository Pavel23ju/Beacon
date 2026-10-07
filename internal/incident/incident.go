// Package incident owns the incident lifecycle. Every state change is
// written to the events table in the same transaction as the row update,
// so the event log is a complete, replayable history.
package incident

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Pavel23ju/beacon/pkg/events"
	"github.com/google/uuid"
)

// Incident is the current-state projection of an incident.
type Incident struct {
	ID             uuid.UUID `json:"id"`
	Type           string    `json:"type"`
	Priority       int16     `json:"priority"` // 0 = untriaged, 1..4 = P1..P4
	Status         Status    `json:"status"`
	Lat            float64   `json:"lat"`
	Lon            float64   `json:"lon"`
	PeopleAffected int       `json:"people_affected"`
	Description    string    `json:"description"`
	ReportedAt     time.Time `json:"reported_at"`
	UpdatedHLC     int64     `json:"updated_hlc"`
}

// NewIncident is the input for reporting an incident.
type NewIncident struct {
	Type           string  `json:"type"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	PeopleAffected int     `json:"people_affected"`
	Description    string  `json:"description"`
}

// ErrInvalid wraps input validation failures.
var ErrInvalid = errors.New("invalid input")

// ErrNotFound is returned when an incident does not exist.
var ErrNotFound = errors.New("incident not found")

func (n NewIncident) Validate() error {
	switch {
	case n.Type == "":
		return fmt.Errorf("%w: type is required", ErrInvalid)
	case n.Lat < -90 || n.Lat > 90:
		return fmt.Errorf("%w: lat out of range", ErrInvalid)
	case n.Lon < -180 || n.Lon > 180:
		return fmt.Errorf("%w: lon out of range", ErrInvalid)
	case n.PeopleAffected < 0:
		return fmt.Errorf("%w: people_affected must be >= 0", ErrInvalid)
	}
	return nil
}

// ValidatePriority checks a priority supplied with a transition. Priority is
// set exactly once, when the incident is triaged.
func ValidatePriority(to Status, priority int16) error {
	if to == Triaged && (priority < 1 || priority > 4) {
		return fmt.Errorf("%w: triage requires priority 1..4", ErrInvalid)
	}
	if to != Triaged && priority != 0 {
		return fmt.Errorf("%w: priority can only be set when triaging", ErrInvalid)
	}
	return nil
}

// reportedPayload is the payload of an incident.reported event.
type reportedPayload struct {
	NewIncident
	ReportedAt time.Time `json:"reported_at"`
}

// transitionedPayload is the payload of an incident.transitioned event.
type transitionedPayload struct {
	From     Status `json:"from"`
	To       Status `json:"to"`
	Priority int16  `json:"priority,omitempty"`
}

// Replay rebuilds an incident from its events, which must be in HLC order.
// It validates every transition, so a corrupt log is detected rather than
// silently producing a wrong state.
func Replay(evs []events.Event) (Incident, error) {
	if len(evs) == 0 {
		return Incident{}, ErrNotFound
	}
	var inc Incident
	for i, ev := range evs {
		switch ev.Type {
		case events.IncidentReported:
			if i != 0 {
				return Incident{}, fmt.Errorf("event %d: reported event must come first", i)
			}
			var p reportedPayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				return Incident{}, fmt.Errorf("event %d: %w", i, err)
			}
			inc = Incident{
				ID: ev.AggregateID, Type: p.Type, Status: Reported,
				Lat: p.Lat, Lon: p.Lon, PeopleAffected: p.PeopleAffected,
				Description: p.Description, ReportedAt: p.ReportedAt,
			}
		case events.IncidentTransitioned:
			if i == 0 {
				return Incident{}, fmt.Errorf("event 0: transition before report")
			}
			var p transitionedPayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				return Incident{}, fmt.Errorf("event %d: %w", i, err)
			}
			if p.From != inc.Status {
				return Incident{}, fmt.Errorf("event %d: from %s but state is %s", i, p.From, inc.Status)
			}
			if err := CanTransition(p.From, p.To); err != nil {
				return Incident{}, fmt.Errorf("event %d: %w", i, err)
			}
			inc.Status = p.To
			if p.To == Triaged {
				inc.Priority = p.Priority
			}
		default:
			return Incident{}, fmt.Errorf("event %d: unknown type %q", i, ev.Type)
		}
		inc.UpdatedHLC = ev.HLC
	}
	return inc, nil
}
