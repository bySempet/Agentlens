// Package httpapi expone la lectura de trazas como API REST (E3-T01).
//
//	GET /healthz               liveness, sin auth
//	GET /v1/traces             listado paginado del tenant autenticado
//	GET /v1/traces/{trace_id}  detalle (spans) de una traza
//
// La auth es por API key (cabecera X-AgentLens-Key), igual que en el gateway de
// ingesta: la key resuelve al tenant y TODA consulta queda acotada a ese tenant;
// el cliente no puede pedir datos de otro.
package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/bysempet/agentlens/api/internal/store"
)

// HeaderAPIKey es la cabecera de autenticación, la misma que emite el SDK.
const HeaderAPIKey = "X-AgentLens-Key"

const (
	defaultLimit = 50
	maxLimit     = 200
)

type api struct {
	store store.Store
	keys  map[string]string // API key -> tenant ID
}

// NewHandler construye el router con auth. keys mapea API key -> tenant.
func NewHandler(st store.Store, keys map[string]string) http.Handler {
	a := &api{store: st, keys: keys}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /v1/traces", a.authenticated(a.listTraces))
	mux.Handle("GET /v1/traces/{trace_id}", a.authenticated(a.getTrace))
	return mux
}

// authenticated resuelve la API key a un tenant y se lo pasa al handler.
func (a *api) authenticated(next func(w http.ResponseWriter, r *http.Request, tenantID string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(HeaderAPIKey)
		if key == "" {
			writeError(w, http.StatusUnauthorized, "falta la cabecera "+HeaderAPIKey)
			return
		}
		tenantID, ok := a.keys[key]
		if !ok {
			writeError(w, http.StatusUnauthorized, "API key inválida")
			return
		}
		next(w, r, tenantID)
	})
}

// listResponse es la respuesta de GET /v1/traces.
type listResponse struct {
	Traces []store.TraceSummary `json:"traces"`
	// NextCursor es opaco; presente solo si hay más páginas.
	NextCursor string `json:"next_cursor,omitempty"`
}

func (a *api) listTraces(w http.ResponseWriter, r *http.Request, tenantID string) {
	q := store.ListQuery{TenantID: tenantID, AgentID: r.URL.Query().Get("agent_id")}

	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "limit debe ser un entero >= 1")
			return
		}
		limit = min(n, maxLimit)
	}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		ts, id, err := decodeCursor(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "cursor inválido")
			return
		}
		q.BeforeStartUnixNano, q.BeforeTraceID = ts, id
	}

	// Se pide uno más que la página para saber si hay siguiente.
	q.Limit = limit + 1
	traces, err := a.store.ListTraces(r.Context(), q)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	resp := listResponse{Traces: traces}
	if len(traces) > limit {
		resp.Traces = traces[:limit]
		last := resp.Traces[limit-1]
		resp.NextCursor = encodeCursor(last.StartUnixNano, last.TraceID)
	}
	writeJSON(w, http.StatusOK, resp)
}

// traceResponse es la respuesta de GET /v1/traces/{trace_id}.
type traceResponse struct {
	TraceID string       `json:"trace_id"`
	Spans   []store.Span `json:"spans"`
}

func (a *api) getTrace(w http.ResponseWriter, r *http.Request, tenantID string) {
	traceID := r.PathValue("trace_id")
	spans, err := a.store.TraceSpans(r.Context(), tenantID, traceID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if len(spans) == 0 {
		writeError(w, http.StatusNotFound, "traza no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, traceResponse{TraceID: traceID, Spans: spans})
}

// encodeCursor serializa la posición de paginación como token opaco.
func encodeCursor(startUnixNano int64, traceID string) string {
	raw := strconv.FormatInt(startUnixNano, 10) + "|" + traceID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor invierte encodeCursor.
func decodeCursor(cursor string) (startUnixNano int64, traceID string, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", err
	}
	tsStr, id, ok := strings.Cut(string(raw), "|")
	if !ok || id == "" {
		return 0, "", errors.New("cursor mal formado")
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return 0, "", err
	}
	return ts, id, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeInternalError registra el detalle en el servidor y devuelve un 500
// genérico: los errores del almacén no se filtran al cliente.
func writeInternalError(w http.ResponseWriter, err error) {
	log.Printf("error del store: %v", err)
	writeError(w, http.StatusInternalServerError, "error interno")
}
