package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KKloudTarus/synapse-ce/internal/domain/qualitygate"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
)

// QualityGateStore persists tenant-scoped custom quality gates.
type QualityGateStore struct{ pool *pgxpool.Pool }

func NewQualityGateStore(pool *pgxpool.Pool) *QualityGateStore { return &QualityGateStore{pool: pool} }

var _ ports.QualityGateStore = (*QualityGateStore)(nil)

func (s *QualityGateStore) Create(ctx context.Context, tenantID shared.ID, gate qualitygate.Gate) error {
	conditions, err := json.Marshal(gate.Conditions)
	if err != nil {
		return fmt.Errorf("marshal quality gate conditions: %w", err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO quality_gates (tenant_id, key, name, conditions) VALUES ($1,$2,$3,$4)`, tenantID.String(), gate.Key, gate.Name, conditions)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return shared.ErrConflict
		}
		return fmt.Errorf("insert quality gate: %w", err)
	}
	return nil
}

func (s *QualityGateStore) List(ctx context.Context, tenantID shared.ID) ([]qualitygate.Gate, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, name, conditions FROM quality_gates WHERE tenant_id=$1 ORDER BY key`, tenantID.String())
	if err != nil {
		return nil, fmt.Errorf("list quality gates: %w", err)
	}
	defer rows.Close()
	out := make([]qualitygate.Gate, 0)
	for rows.Next() {
		gate, err := scanQualityGate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, gate)
	}
	return out, rows.Err()
}

func (s *QualityGateStore) Get(ctx context.Context, tenantID shared.ID, key string) (qualitygate.Gate, error) {
	gate, err := scanQualityGate(s.pool.QueryRow(ctx, `SELECT key, name, conditions FROM quality_gates WHERE tenant_id=$1 AND key=$2`, tenantID.String(), key))
	if errors.Is(err, pgx.ErrNoRows) {
		return qualitygate.Gate{}, shared.ErrNotFound
	}
	if err != nil {
		return qualitygate.Gate{}, fmt.Errorf("select quality gate: %w", err)
	}
	return gate, nil
}

func (s *QualityGateStore) Update(ctx context.Context, tenantID shared.ID, gate qualitygate.Gate) error {
	conditions, err := json.Marshal(gate.Conditions)
	if err != nil {
		return fmt.Errorf("marshal quality gate conditions: %w", err)
	}
	ct, err := s.pool.Exec(ctx, `UPDATE quality_gates SET name=$3, conditions=$4, updated_at=now() WHERE tenant_id=$1 AND key=$2`, tenantID.String(), gate.Key, gate.Name, conditions)
	if err != nil {
		return fmt.Errorf("update quality gate: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *QualityGateStore) Delete(ctx context.Context, tenantID shared.ID, key string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM quality_gates WHERE tenant_id=$1 AND key=$2`, tenantID.String(), key)
	if err != nil {
		return fmt.Errorf("delete quality gate: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return shared.ErrNotFound
	}
	return nil
}

type qualityGateScanner interface{ Scan(...any) error }

func scanQualityGate(row qualityGateScanner) (qualitygate.Gate, error) {
	var gate qualitygate.Gate
	var conditions []byte
	if err := row.Scan(&gate.Key, &gate.Name, &conditions); err != nil {
		return qualitygate.Gate{}, err
	}
	if err := json.Unmarshal(conditions, &gate.Conditions); err != nil {
		return qualitygate.Gate{}, fmt.Errorf("unmarshal quality gate conditions: %w", err)
	}
	return gate, nil
}
