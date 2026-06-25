// Package httpapi expone la API hot path de lectura de trazas (E3-T01):
// listado y detalle con paginación, sobre un TraceStore.
package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bysempet/agentlens/api/internal/store"
)

// TenantHeader es la cabecera de la que se lee el tenant. En producción la
// fijará el gateway/auth tras validar la sesión; aquí es el punto de entrada.
const TenantHeader = "X-AgentLens-Tenant"

const (
	defaultLimit = 50
	maxLimit     = 200
)

// Handler enruta las peticiones de la API contra un TraceStore.
type Handler struct {
	store store.TraceStore
	mux   *http.ServeMux
}

// New construye el handler y registra las rutas.
func New(s store.TraceStore) *Handler {
	h := &Handler{store: s, mux: http.NewServeMux()}
	h.mux.HandleFunc("GET /v1/traces", h.listTraces)
	h.mux.HandleFunc("GET /v1/traces/{traceId}", h.getTrace)
	h.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.mux.ServeHTTP(w, r) }

func (h *Handler) listTraces(w http.ResponseWriter, r *http.Request) {
	tenant := r.Header.Get(TenantHeader)
	if tenant == "" {
		writeError(w, http.StatusBadRequest, "falta la cabecera "+TenantHeader)
		return
	}
	page, err := parsePage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	traces, err := h.store.ListTraces(r.Context(), tenant, page)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error consultando trazas")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"traces": traces,
		"limit":  page.Limit,
		"offset": page.Offset,
		"count":  len(traces),
	})
}

func (h *Handler) getTrace(w http.ResponseWriter, r *http.Request) {
	tenant := r.Header.Get(TenantHeader)
	if tenant == "" {
		writeError(w, http.StatusBadRequest, "falta la cabecera "+TenantHeader)
		return
	}
	traceID := r.PathValue("traceId")
	spans, err := h.store.GetTrace(r.Context(), tenant, traceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error consultando la traza")
		return
	}
	if len(spans) == 0 {
		writeError(w, http.StatusNotFound, "traza no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"trace_id": traceID,
		"spans":    spans,
	})
}

// parsePage valida limit/offset con defaults y tope máximo.
func parsePage(r *http.Request) (store.Page, error) {
	limit := defaultLimit
	offset := 0
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return store.Page{}, errBadParam("limit")
		}
		limit = n
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return store.Page{}, errBadParam("offset")
		}
		offset = n
	}
	return store.Page{Limit: limit, Offset: offset}, nil
}

type paramError struct{ name string }

func (e paramError) Error() string { return "parámetro inválido: " + e.name }

func errBadParam(name string) error { return paramError{name} }

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
