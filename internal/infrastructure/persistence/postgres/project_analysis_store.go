package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KKloudTarus/synapse-ce/internal/domain/projectanalysis"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
)

// ProjectAnalysisStore persists immutable Project analysis snapshots.
type ProjectAnalysisStore struct{ pool *pgxpool.Pool }

func NewProjectAnalysisStore(pool *pgxpool.Pool) *ProjectAnalysisStore {
	return &ProjectAnalysisStore{pool: pool}
}

var _ ports.ProjectAnalysisStore = (*ProjectAnalysisStore)(nil)

func (r *ProjectAnalysisStore) Save(ctx context.Context, analysis projectanalysis.Analysis) error {
	payload, err := json.Marshal(analysis)
	if err != nil {
		return fmt.Errorf("marshal project analysis: %w", err)
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO project_analyses (id, tenant_id, project_id, created_at, payload)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO NOTHING`,
		analysis.ID, analysis.TenantID, analysis.ProjectID, analysis.CreatedAt, payload)
	if err != nil {
		return fmt.Errorf("insert project analysis: %w", err)
	}
	return nil
}

func (r *ProjectAnalysisStore) List(ctx context.Context, tenantID, projectID shared.ID, limit int, beforeCreatedAt time.Time, beforeID shared.ID) ([]projectanalysis.Analysis, bool, error) {
	var (
		rows pgx.Rows
		err  error
	)
	cursor := beforeCreatedAt
	if beforeCreatedAt.IsZero() {
		cursor = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}
	if tenantID.IsZero() {
		rows, err = r.pool.Query(ctx, `SELECT payload FROM project_analyses
			WHERE project_id=$1 AND (created_at < $2 OR (created_at = $2 AND id COLLATE "C" < $3))
			ORDER BY created_at DESC, id COLLATE "C" DESC LIMIT $4`, projectID.String(), cursor, beforeID.String(), limit+1)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT payload FROM project_analyses
			WHERE tenant_id=$1 AND project_id=$2 AND (created_at < $3 OR (created_at = $3 AND id COLLATE "C" < $4))
			ORDER BY created_at DESC, id COLLATE "C" DESC LIMIT $5`, tenantID.String(), projectID.String(), cursor, beforeID.String(), limit+1)
	}
	if err != nil {
		return nil, false, fmt.Errorf("list project analyses: %w", err)
	}
	defer rows.Close()
	out := make([]projectanalysis.Analysis, 0, limit+1)
	for rows.Next() {
		analysis, err := scanProjectAnalysis(rows)
		if err != nil {
			return nil, false, err
		}
		out = append(out, analysis)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}

func (r *ProjectAnalysisStore) Get(ctx context.Context, tenantID, projectID, analysisID shared.ID) (projectanalysis.Analysis, error) {
	var row pgx.Row
	if tenantID.IsZero() {
		row = r.pool.QueryRow(ctx, `SELECT payload FROM project_analyses WHERE project_id=$1 AND id=$2`, projectID.String(), analysisID.String())
	} else {
		row = r.pool.QueryRow(ctx, `SELECT payload FROM project_analyses WHERE tenant_id=$1 AND project_id=$2 AND id=$3`, tenantID.String(), projectID.String(), analysisID.String())
	}
	analysis, err := scanProjectAnalysis(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return projectanalysis.Analysis{}, shared.ErrNotFound
	}
	if err != nil {
		return projectanalysis.Analysis{}, fmt.Errorf("get project analysis: %w", err)
	}
	return analysis, nil
}

func scanProjectAnalysis(row rowScanner) (projectanalysis.Analysis, error) {
	var payload []byte
	if err := row.Scan(&payload); err != nil {
		return projectanalysis.Analysis{}, err
	}
	var analysis projectanalysis.Analysis
	if err := json.Unmarshal(payload, &analysis); err != nil {
		return projectanalysis.Analysis{}, fmt.Errorf("decode project analysis: %w", err)
	}
	return analysis, nil
}
