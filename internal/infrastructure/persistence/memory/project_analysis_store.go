package memory

import (
	"context"
	"maps"
	"slices"
	"sort"
	"sync"

	"github.com/KKloudTarus/synapse-ce/internal/domain/measure"
	"time"

	"github.com/KKloudTarus/synapse-ce/internal/domain/projectanalysis"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
)

// ProjectAnalysisStore is an append-only in-memory Project analysis store.
type ProjectAnalysisStore struct {
	mu   sync.RWMutex
	data []projectanalysis.Analysis
}

func NewProjectAnalysisStore() *ProjectAnalysisStore { return &ProjectAnalysisStore{} }

var _ ports.ProjectAnalysisStore = (*ProjectAnalysisStore)(nil)

func (s *ProjectAnalysisStore) Save(_ context.Context, analysis projectanalysis.Analysis) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, current := range s.data {
		if current.ID == analysis.ID {
			return nil
		}
	}
	s.data = append(s.data, cloneProjectAnalysis(analysis))
	return nil
}

func (s *ProjectAnalysisStore) List(_ context.Context, tenantID, projectID shared.ID, limit int, beforeCreatedAt time.Time, beforeID shared.ID) ([]projectanalysis.Analysis, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]projectanalysis.Analysis, 0)
	for _, analysis := range s.data {
		if analysis.ProjectID != projectID.String() || (!tenantID.IsZero() && analysis.TenantID != tenantID.String()) {
			continue
		}
		if !beforeCreatedAt.IsZero() && (analysis.CreatedAt.After(beforeCreatedAt) || (analysis.CreatedAt.Equal(beforeCreatedAt) && analysis.ID >= beforeID.String())) {
			continue
		}
		out = append(out, cloneProjectAnalysis(analysis))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}

func cloneProjectAnalysis(in projectanalysis.Analysis) projectanalysis.Analysis {
	out := in
	out.Measures = maps.Clone(in.Measures)
	out.Gate.Results = slices.Clone(in.Gate.Results)
	out.InternalIssues = slices.Clone(in.InternalIssues)
	out.Issues = cloneCounts(in.Issues)
	out.NewCode.Counts = cloneCounts(in.NewCode.Counts)
	if in.Delta != nil {
		delta := *in.Delta
		delta.Issues = cloneCounts(in.Delta.Issues)
		delta.Measures = maps.Clone(in.Delta.Measures)
		delta.Ratings = maps.Clone(in.Delta.Ratings)
		out.Delta = &delta
	}
	if in.Coverage != nil {
		coverage := *in.Coverage
		coverage.Files = slices.Clone(in.Coverage.Files)
		out.Coverage = &coverage
	}
	out.Duplication = cloneDuplication(in.Duplication)
	return out
}

func cloneCounts(in projectanalysis.Counts) projectanalysis.Counts {
	out := in
	out.ByKind = maps.Clone(in.ByKind)
	out.BySeverity = maps.Clone(in.BySeverity)
	out.ByStatus = maps.Clone(in.ByStatus)
	return out
}

func cloneDuplication(in measure.DuplicationReport) measure.DuplicationReport {
	out := in
	out.Blocks = slices.Clone(in.Blocks)
	for i := range out.Blocks {
		out.Blocks[i].Occurrences = slices.Clone(in.Blocks[i].Occurrences)
	}
	return out
}

func (s *ProjectAnalysisStore) Get(_ context.Context, tenantID, projectID, analysisID shared.ID) (projectanalysis.Analysis, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, analysis := range s.data {
		if analysis.ID == analysisID.String() && analysis.ProjectID == projectID.String() && (tenantID.IsZero() || analysis.TenantID == tenantID.String()) {
			return cloneProjectAnalysis(analysis), nil
		}
	}
	return projectanalysis.Analysis{}, shared.ErrNotFound
}
