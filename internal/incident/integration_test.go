//go:build integration

package incident_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/Pavel23ju/beacon/internal/incident"
	"github.com/Pavel23ju/beacon/internal/testdb"
	"github.com/Pavel23ju/beacon/pkg/events"
	"github.com/Pavel23ju/beacon/pkg/hlc"
	"github.com/go-chi/chi/v5"
)

var server *httptest.Server

func TestMain(m *testing.M) {
	pool, cleanup := testdb.Start()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := incident.NewHandler(incident.NewStore(pool, hlc.NewClock(), "test-node"), log)
	r := chi.NewRouter()
	r.Route("/incidents", h.Routes)
	server = httptest.NewServer(r)

	code := m.Run()
	server.Close()
	cleanup()
	os.Exit(code)
}

func call(t *testing.T, method, path string, body any, out any) int {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, server.URL+path, rd)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

func create(t *testing.T) incident.Incident {
	t.Helper()
	var inc incident.Incident
	code := call(t, "POST", "/incidents", incident.NewIncident{
		Type: "building_collapse", Lat: 22.5726, Lon: 88.3639, PeopleAffected: 3,
		Description: "building collapse, 3 injured",
	}, &inc)
	if code != http.StatusCreated {
		t.Fatalf("create: status %d", code)
	}
	return inc
}

func move(t *testing.T, inc incident.Incident, to incident.Status, prio int16) int {
	t.Helper()
	return call(t, "POST", "/incidents/"+inc.ID.String()+"/transition",
		map[string]any{"to": to, "priority": prio}, nil)
}

func TestFullLifecycleAndReplay(t *testing.T) {
	inc := create(t)
	if inc.Status != incident.Reported || inc.Lat != 22.5726 || inc.Lon != 88.3639 {
		t.Fatalf("unexpected created incident: %+v", inc)
	}

	if code := move(t, inc, incident.Dispatched, 0); code != http.StatusConflict {
		t.Fatalf("illegal jump REPORTED->DISPATCHED: want 409, got %d", code)
	}

	steps := []struct {
		to   incident.Status
		prio int16
	}{
		{incident.Triaged, 1}, {incident.Dispatched, 0}, {incident.Reassigned, 0},
		{incident.Dispatched, 0}, {incident.Acknowledged, 0}, {incident.EnRoute, 0},
		{incident.OnScene, 0}, {incident.Resolved, 0},
	}
	for _, s := range steps {
		if code := move(t, inc, s.to, s.prio); code != http.StatusOK {
			t.Fatalf("-> %s: status %d", s.to, code)
		}
	}
	if code := move(t, inc, incident.Reported, 0); code != http.StatusConflict {
		t.Fatalf("transition out of RESOLVED: want 409, got %d", code)
	}

	var current incident.Incident
	call(t, "GET", "/incidents/"+inc.ID.String(), nil, &current)
	var evs []events.Event
	call(t, "GET", "/incidents/"+inc.ID.String()+"/events", nil, &evs)

	if len(evs) != 1+len(steps) {
		t.Fatalf("want %d events, got %d (rejected transitions must not be logged)", 1+len(steps), len(evs))
	}
	for i := 1; i < len(evs); i++ {
		if evs[i].HLC <= evs[i-1].HLC {
			t.Fatalf("events not strictly HLC-ordered at %d", i)
		}
	}

	replayed, err := incident.Replay(evs)
	if err != nil {
		t.Fatal(err)
	}
	// reported_at round-trips through JSON; compare instants, not monotonic clocks.
	if !replayed.ReportedAt.Equal(current.ReportedAt) {
		t.Fatalf("reported_at mismatch: %v vs %v", replayed.ReportedAt, current.ReportedAt)
	}
	replayed.ReportedAt = current.ReportedAt
	if replayed != current {
		t.Fatalf("replay diverges from stored state:\nreplay: %+v\nstored: %+v", replayed, current)
	}
	if current.Status != incident.Resolved || current.Priority != 1 {
		t.Fatalf("unexpected final state: %+v", current)
	}
}

// Many clients race the same transition; row locking must let exactly one win.
func TestConcurrentTransitionsSerialize(t *testing.T) {
	inc := create(t)
	move(t, inc, incident.Triaged, 2)
	move(t, inc, incident.Dispatched, 0)

	const n = 50
	var wg sync.WaitGroup
	codes := make(chan int, n)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- move(t, inc, incident.Acknowledged, 0)
		}()
	}
	wg.Wait()
	close(codes)

	ok, conflict := 0, 0
	for c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Errorf("unexpected status %d", c)
		}
	}
	if ok != 1 || conflict != n-1 {
		t.Fatalf("want 1 success and %d conflicts, got %d and %d", n-1, ok, conflict)
	}
}

func TestNotFoundAndBadInput(t *testing.T) {
	if code := call(t, "GET", "/incidents/00000000-0000-0000-0000-000000000000", nil, nil); code != http.StatusNotFound {
		t.Errorf("unknown id: want 404, got %d", code)
	}
	if code := call(t, "GET", "/incidents/not-a-uuid", nil, nil); code != http.StatusBadRequest {
		t.Errorf("bad id: want 400, got %d", code)
	}
	if code := call(t, "POST", "/incidents", map[string]any{"lat": 200}, nil); code != http.StatusBadRequest {
		t.Errorf("invalid body: want 400, got %d", code)
	}
	inc := create(t)
	if code := move(t, inc, incident.Triaged, 0); code != http.StatusBadRequest {
		t.Errorf("triage without priority: want 400, got %d", code)
	}
}
