package responder

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/Pavel23ju/beacon/internal/platform"
	"github.com/go-chi/chi/v5"
)

// Handler exposes the responder REST API.
type Handler struct {
	store *Store
	log   *slog.Logger
}

func NewHandler(store *Store, log *slog.Logger) *Handler {
	return &Handler{store: store, log: log}
}

// Routes mounts under /responders.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/", h.register)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var in NewResponder
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		platform.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	resp, err := h.store.Register(r.Context(), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := platform.URLParamUUID(w, r, "id")
	if !ok {
		return
	}
	resp, err := h.store.Get(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && !slices.Contains([]string{Available, Busy, Offline}, status) {
		platform.WriteError(w, http.StatusBadRequest, "unknown status filter")
		return
	}
	list, err := h.store.List(r.Context(), status)
	if err != nil {
		h.fail(w, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		platform.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrInvalid):
		platform.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		h.log.Error("responder request failed", "err", err)
		platform.WriteError(w, http.StatusInternalServerError, "internal error")
	}
}
