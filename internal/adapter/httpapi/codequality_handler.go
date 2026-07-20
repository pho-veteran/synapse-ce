package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/codequality"
)

// codeQualityService is the narrow slice of the code-quality use-case the HTTP layer needs: build the
// dashboard report (inventory + findings + duplication + ratings) for a local source tree.
// *codequality.Service satisfies it. Optional: nil => the code-quality route is not registered.
type codeQualityService interface {
	BuildReport(ctx context.Context, root string) (codequality.Report, error)
}

// SetCodeQuality wires the read-only code-quality dashboard endpoint.
func (rt *Router) SetCodeQuality(s codeQualityService) { rt.codeQuality = s }

// codeQualityReportView wraps the latest stored code-quality report with an availability flag.
type codeQualityReportView struct {
	Available bool                `json:"available"`
	Reason    string              `json:"reason,omitempty"`
	Report    *codequality.Report `json:"report,omitempty"`
}

const codeQualityUnavailable = "Run an Engagement scan with Code quality enabled to generate a stored report"

// codeQualityReport returns the code-quality report stored by the latest explicit Engagement scan. Auth
// (PermView) + tenant scoping are applied by the route wrapper. Reading this endpoint never analyzes source.
func (rt *Router) codeQualityReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "engagement id is required"})
		return
	}
	if _, err := rt.eng.Get(r.Context(), shared.ID(TenantFrom(r.Context())), shared.ID(id)); err != nil {
		writeError(w, rt.log, err)
		return
	}
	data, err := rt.sca.LatestResult(r.Context(), shared.ID(id))
	if errors.Is(err, shared.ErrNotFound) {
		writeJSON(w, http.StatusOK, codeQualityReportView{Reason: codeQualityUnavailable})
		return
	}
	if err != nil {
		writeError(w, rt.log, err)
		return
	}
	var cached struct {
		CodeQuality *codequality.Report `json:"code_quality"`
	}
	if err := json.Unmarshal(data, &cached); err != nil {
		writeError(w, rt.log, err)
		return
	}
	if cached.CodeQuality == nil {
		writeJSON(w, http.StatusOK, codeQualityReportView{Reason: codeQualityUnavailable})
		return
	}
	writeJSON(w, http.StatusOK, codeQualityReportView{Available: true, Report: cached.CodeQuality})
}
