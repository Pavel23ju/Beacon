//go:build integration

package responder_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"github.com/Pavel23ju/beacon/internal/responder"
	"github.com/Pavel23ju/beacon/internal/testdb"
	"github.com/Pavel23ju/beacon/pkg/hlc"
	"github.com/go-chi/chi/v5"
)

var server *httptest.Server

func TestMain(m *testing.M) {
	pool, cleanup := testdb.Start()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := responder.NewHandler(responder.NewStore(pool, hlc.NewClock(), "test-node"), log)
	r := chi.NewRouter()
	r.Route("/responders", h.Routes)
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

func TestRegisterGetList(t *testing.T) {
	var amb responder.Responder
	code := call(t, "POST", "/responders", responder.NewResponder{
		Type: responder.Ambulance, Capabilities: []string{"trauma", "als", "trauma"},
	}, &amb)
	if code != http.StatusCreated {
		t.Fatalf("register: status %d", code)
	}
	if amb.Status != responder.Available || amb.Mode != "online" || amb.Lat != nil || amb.LastSeen != nil {
		t.Fatalf("unexpected new responder: %+v", amb)
	}
	if !slices.Equal(amb.Capabilities, []string{"als", "trauma"}) {
		t.Fatalf("capabilities should be sorted and deduplicated, got %v", amb.Capabilities)
	}

	var bare responder.Responder
	call(t, "POST", "/responders", responder.NewResponder{Type: responder.Fire}, &bare)
	if bare.Capabilities == nil {
		t.Fatal("capabilities should be [] not null")
	}

	var got responder.Responder
	if code := call(t, "GET", "/responders/"+amb.ID.String(), nil, &got); code != http.StatusOK {
		t.Fatalf("get: status %d", code)
	}
	if got.ID != amb.ID || got.Type != responder.Ambulance {
		t.Fatalf("get mismatch: %+v", got)
	}

	var list []responder.Responder
	call(t, "GET", "/responders?status=AVAILABLE", nil, &list)
	ids := map[string]bool{}
	for _, r := range list {
		ids[r.ID.String()] = true
	}
	if !ids[amb.ID.String()] || !ids[bare.ID.String()] {
		t.Fatalf("list missing registered responders: %v", list)
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		method, path string
		body         any
		want         int
	}{
		{"POST", "/responders", map[string]any{"type": "helicopter"}, http.StatusBadRequest},
		{"GET", "/responders/not-a-uuid", nil, http.StatusBadRequest},
		{"GET", "/responders/00000000-0000-0000-0000-000000000000", nil, http.StatusNotFound},
		{"GET", "/responders?status=SLEEPING", nil, http.StatusBadRequest},
	}
	for _, c := range cases {
		if got := call(t, c.method, c.path, c.body, nil); got != c.want {
			t.Errorf("%s %s: want %d, got %d", c.method, c.path, c.want, got)
		}
	}
}
