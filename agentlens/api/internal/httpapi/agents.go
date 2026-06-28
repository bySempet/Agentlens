package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bysempet/agentlens/api/internal/agents"
	"github.com/bysempet/agentlens/api/internal/auth"
)

func (h *handler) createAgent(w http.ResponseWriter, r *http.Request) {
	tenant, ok := auth.TenantFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	var in agents.CreateInput
	// Límite de tamaño del cuerpo (defensa): el registro de un agente es pequeño.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "el nombre es obligatorio")
		return
	}
	a, err := h.agents.Create(r.Context(), tenant, in)
	if err != nil {
		if err == agents.ErrTenantNotFound {
			writeError(w, http.StatusBadRequest, "tenant no provisionado")
			return
		}
		writeError(w, http.StatusInternalServerError, "no se pudo registrar el agente")
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (h *handler) listAgents(w http.ResponseWriter, r *http.Request) {
	tenant, ok := auth.TenantFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	list, err := h.agents.List(r.Context(), tenant)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo listar")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": list, "count": len(list)})
}

func (h *handler) getAgent(w http.ResponseWriter, r *http.Request) {
	tenant, ok := auth.TenantFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	a, err := h.agents.Get(r.Context(), tenant, r.PathValue("agentId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error consultando el agente")
		return
	}
	if a == nil {
		writeError(w, http.StatusNotFound, "agente no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *handler) deleteAgent(w http.ResponseWriter, r *http.Request) {
	tenant, ok := auth.TenantFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	ok, err := h.agents.Disable(r.Context(), tenant, r.PathValue("agentId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo dar de baja")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "agente no encontrado")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
