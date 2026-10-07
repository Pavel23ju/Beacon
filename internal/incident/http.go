package incident

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Pavel23ju/beacon/internal/platform"
	"github.com/go-chi/chi/v5"
)

// Handler exposes the incident REST API.
type Handler struct {
	store *Store
	log   *slog.Logger
}

func NewHandler(store *Store, log *slog.Logger) *Handler {
	return &Handler{store: store, log: log}
}

// Routes mounts under /incidents.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Post("/{id}/transition", h.transition)
	r.Get("/{id}/events", h.events)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in NewIncident
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	inc, err := h.store.Create(r.Context(), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusCreated, inc)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := platform.URLParamUUID(w, r, "id")
	if !ok {
		return
	}
	inc, err := h.store.Get(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, inc)
}

type transitionRequest struct {
	To       Status `json:"to"`
	Priority int16  `json:"priority,omitempty"`
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request) {
	id, ok := platform.URLParamUUID(w, r, "id")
	if !ok {
		return
	}
	var req transitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	inc, err := h.store.Transition(r.Context(), id, req.To, req.Priority)
	if err != nil {
		h.fail(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, inc)
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	id, ok := platform.URLParamUUID(w, r, "id")
	if !ok {
		return
	}
	evs, err := h.store.Events(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	if len(evs) == 0 {
		h.fail(w, ErrNotFound)
		return
	}
	platform.WriteJSON(w, http.StatusOK, evs)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	var illegal ErrIllegalTransition
	switch {
	case errors.Is(err, ErrNotFound):
		platform.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrInvalid):
		platform.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.As(err, &illegal):
		platform.WriteError(w, http.StatusConflict, err.Error())
	default:
		h.log.Error("incident request failed", "err", err)
		platform.WriteError(w, http.StatusInternalServerError, "internal error")
	}
}
