// Package httpapi expone la API hot path de lectura de trazas (E3-T01):
// listado y detalle con paginación, sobre un TraceStore.
package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/bysempet/agentlens/api/internal/auth"
	"github.com/bysempet/agentlens/api/internal/cost"
	"github.com/bysempet/agentlens/api/internal/store"
	"github.com/bysempet/agentlens/api/spec"
)

const (
	defaultLimit          = 50
	maxLimit              = 200
	defaultStreamInterval = time.Second
)

// Option configura el handler.
type Option func(*handler)

// WithStreamInterval ajusta el periodo de sondeo del live feed (E3-T05).
func WithStreamInterval(d time.Duration) Option {
	return func(h *handler) { h.streamInterval = d }
}

// New construye el handler HTTP: registra las rutas y las envuelve con la
// autenticación por API key. /healthz y /openapi.yaml son públicas.
func New(s store.TraceStore, keys auth.KeyStore, opts ...Option) http.Handler {
	h := &handler{store: s, streamInterval: defaultStreamInterval}
	for _, opt := range opts {
		opt(h)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/traces", h.listTraces)
	mux.HandleFunc("GET /v1/traces/{traceId}", h.getTrace)
	mux.HandleFunc("GET /v1/cost", h.getCost)
	mux.HandleFunc("GET /v1/stream", h.stream)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(spec.OpenAPIYAML)
	})
	return auth.Middleware(mux, keys, "/healthz", "/openapi.yaml")
}

// handler agrupa los manejadores sobre un TraceStore.
type handler struct {
	store          store.TraceStore
	streamInterval time.Duration
}

func (h *handler) listTraces(w http.ResponseWriter, r *http.Request) {
	tenant, ok := auth.TenantFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
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

func (h *handler) getTrace(w http.ResponseWriter, r *http.Request) {
	tenant, ok := auth.TenantFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
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

func (h *handler) getCost(w http.ResponseWriter, r *http.Request) {
	tenant, ok := auth.TenantFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	rows, err := h.store.CostRows(r.Context(), tenant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error consultando el coste")
		return
	}
	writeJSON(w, http.StatusOK, cost.Summarize(rows))
}

// stream emite por WebSocket las trazas nuevas del tenant en tiempo real
// (live feed, E3-T05). Sondea el almacén y empuja cada traza iniciada tras la
// conexión. Solo trazas del tenant autenticado (aislamiento).
func (h *handler) stream(w http.ResponseWriter, r *http.Request) {
	tenant, ok := auth.TenantFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	ctx := conn.CloseRead(r.Context()) // detecta cierre del cliente
	since := time.Now()
	ticker := time.NewTicker(h.streamInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			traces, err := h.store.ListTracesSince(ctx, tenant, since, 100)
			if err != nil {
				return
			}
			for _, t := range traces {
				if t.StartTime.After(since) {
					since = t.StartTime
				}
				if err := wsjson.Write(ctx, conn, t); err != nil {
					return
				}
			}
		}
	}
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
