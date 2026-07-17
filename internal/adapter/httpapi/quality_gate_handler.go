package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/KKloudTarus/synapse-ce/internal/domain/qualitygate"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
)

func (rt *Router) listQualityGates(w http.ResponseWriter, r *http.Request) {
	gates, err := rt.qualityGates.List(r.Context(), shared.ID(TenantFrom(r.Context())))
	if err != nil {
		writeError(w, rt.log, err)
		return
	}
	writeJSON(w, http.StatusOK, gates)
}

func (rt *Router) getQualityGate(w http.ResponseWriter, r *http.Request) {
	gate, err := rt.qualityGates.Get(r.Context(), shared.ID(TenantFrom(r.Context())), r.PathValue("key"))
	if err != nil {
		writeError(w, rt.log, err)
		return
	}
	writeJSON(w, http.StatusOK, gate)
}

func (rt *Router) createQualityGate(w http.ResponseWriter, r *http.Request) {
	var gate qualitygate.Gate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&gate); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json body"})
		return
	}
	gate, err := rt.qualityGates.Create(r.Context(), PrincipalFrom(r.Context()), shared.ID(TenantFrom(r.Context())), gate)
	if err != nil {
		writeError(w, rt.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, gate)
}

func (rt *Router) updateQualityGate(w http.ResponseWriter, r *http.Request) {
	var gate qualitygate.Gate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&gate); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json body"})
		return
	}
	gate, err := rt.qualityGates.Update(r.Context(), PrincipalFrom(r.Context()), shared.ID(TenantFrom(r.Context())), r.PathValue("key"), gate)
	if err != nil {
		writeError(w, rt.log, err)
		return
	}
	writeJSON(w, http.StatusOK, gate)
}

func (rt *Router) deleteQualityGate(w http.ResponseWriter, r *http.Request) {
	if err := rt.qualityGates.Delete(r.Context(), PrincipalFrom(r.Context()), shared.ID(TenantFrom(r.Context())), r.PathValue("key")); err != nil {
		writeError(w, rt.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
