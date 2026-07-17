package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/KKloudTarus/synapse-ce/internal/domain/project"
	"github.com/KKloudTarus/synapse-ce/internal/domain/qualitygate"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
)

// QualityGateMutator makes managed-gate writes atomic with their audit record in memory.
type QualityGateMutator struct {
	mu       sync.Mutex
	gates    *QualityGateStore
	projects *ProjectRepository
	audit    ports.AuditLogger
}

func NewQualityGateMutator(gates *QualityGateStore, projects *ProjectRepository, audit ports.AuditLogger) *QualityGateMutator {
	return &QualityGateMutator{gates: gates, projects: projects, audit: audit}
}

var _ ports.QualityGateMutator = (*QualityGateMutator)(nil)

func (m *QualityGateMutator) CreateGate(ctx context.Context, tenantID shared.ID, gate qualitygate.Gate, entry ports.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.gates.Get(ctx, tenantID, gate.Key); err == nil {
		return shared.ErrConflict
	} else if err != shared.ErrNotFound {
		return err
	}
	if err := m.audit.Record(ctx, entry); err != nil {
		return err
	}
	return m.gates.Create(ctx, tenantID, gate)
}

func (m *QualityGateMutator) UpdateGate(ctx context.Context, tenantID shared.ID, gate qualitygate.Gate, entry ports.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.gates.Get(ctx, tenantID, gate.Key); err != nil {
		return err
	}
	if err := m.audit.Record(ctx, entry); err != nil {
		return err
	}
	return m.gates.Update(ctx, tenantID, gate)
}

func (m *QualityGateMutator) DeleteGate(ctx context.Context, tenantID shared.ID, key string, entry ports.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.gates.Get(ctx, tenantID, key); err != nil {
		return err
	}
	assigned, err := m.projects.CountByGate(ctx, tenantID, key)
	if err != nil {
		return err
	}
	if assigned > 0 {
		return shared.ErrConflict
	}
	if err := m.audit.Record(ctx, entry); err != nil {
		return err
	}
	return m.gates.Delete(ctx, tenantID, key)
}

func (m *QualityGateMutator) AssignProjectGate(ctx context.Context, tenantID shared.ID, projectKey, gateID string, entry ports.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	gateID = strings.TrimSpace(gateID)
	if err := m.requireCustomGate(ctx, tenantID, gateID); err != nil {
		return err
	}
	if _, err := m.projects.GetByKey(ctx, tenantID, projectKey); err != nil {
		return err
	}
	if err := m.audit.Record(ctx, entry); err != nil {
		return err
	}
	return m.projects.UpdateGate(ctx, tenantID, projectKey, gateID)
}

func (m *QualityGateMutator) CreateProjectWithGate(ctx context.Context, p *project.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.requireCustomGate(ctx, p.TenantID, p.GateID); err != nil {
		return err
	}
	return m.projects.Create(ctx, p)
}

func (m *QualityGateMutator) requireCustomGate(ctx context.Context, tenantID shared.ID, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	if _, builtIn := qualitygate.Resolve(key); builtIn {
		return nil
	}
	_, err := m.gates.Get(ctx, tenantID, key)
	return err
}
