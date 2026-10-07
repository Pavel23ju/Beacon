package incident

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Pavel23ju/beacon/pkg/events"
	"github.com/google/uuid"
)

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func reported(t *testing.T, id uuid.UUID) events.Event {
	return events.Event{
		AggregateID: id, Type: events.IncidentReported, HLC: 1,
		Payload: mustJSON(t, reportedPayload{NewIncident: NewIncident{Type: "fire", Lat: 22.5, Lon: 88.3, PeopleAffected: 3}}),
	}
}

func moved(t *testing.T, id uuid.UUID, hlc int64, from, to Status, prio int16) events.Event {
	return events.Event{
		AggregateID: id, Type: events.IncidentTransitioned, HLC: hlc,
		Payload: mustJSON(t, transitionedPayload{From: from, To: to, Priority: prio}),
	}
}

func TestReplayRebuildsState(t *testing.T) {
	id := uuid.New()
	inc, err := Replay([]events.Event{
		reported(t, id),
		moved(t, id, 2, Reported, Triaged, 1),
		moved(t, id, 3, Triaged, Dispatched, 0),
		moved(t, id, 4, Dispatched, Reassigned, 0),
		moved(t, id, 5, Reassigned, Dispatched, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if inc.ID != id || inc.Status != Dispatched || inc.Priority != 1 || inc.UpdatedHLC != 5 || inc.Type != "fire" {
		t.Fatalf("unexpected state: %+v", inc)
	}
}

func TestReplayRejectsCorruptLogs(t *testing.T) {
	id := uuid.New()
	cases := map[string][]events.Event{
		"empty":            nil,
		"no report":        {moved(t, id, 1, Reported, Triaged, 1)},
		"illegal jump":     {reported(t, id), moved(t, id, 2, Reported, Resolved, 0)},
		"from mismatch":    {reported(t, id), moved(t, id, 2, Triaged, Dispatched, 0)},
		"duplicate report": {reported(t, id), reported(t, id)},
		"unknown type":     {reported(t, id), {AggregateID: id, Type: "bogus", Payload: []byte(`{}`)}},
	}
	for name, evs := range cases {
		if _, err := Replay(evs); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := Replay(nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty log: expected ErrNotFound, got %v", err)
	}
}

func TestValidation(t *testing.T) {
	bad := []NewIncident{
		{Lat: 0, Lon: 0},
		{Type: "fire", Lat: 91},
		{Type: "fire", Lon: -181},
		{Type: "fire", PeopleAffected: -1},
	}
	for _, in := range bad {
		if err := in.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: expected ErrInvalid, got %v", in, err)
		}
	}
	if err := ValidatePriority(Triaged, 0); !errors.Is(err, ErrInvalid) {
		t.Error("triage without priority should be invalid")
	}
	if err := ValidatePriority(Dispatched, 2); !errors.Is(err, ErrInvalid) {
		t.Error("priority outside triage should be invalid")
	}
	if err := ValidatePriority(Triaged, 3); err != nil {
		t.Errorf("valid triage rejected: %v", err)
	}
}
