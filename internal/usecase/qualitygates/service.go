// Package qualitygates manages tenant-scoped quality-gate definitions.
package qualitygates

import (
	"context"
	"fmt"
	"strings"

	"github.com/KKloudTarus/synapse-ce/internal/domain/qualitygate"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
)

type Service struct {
	store   ports.QualityGateStore
	mutator ports.QualityGateMutator
	clock   ports.Clock
}

func NewService(store ports.QualityGateStore, _ ports.AuditLogger, clock ports.Clock) *Service {
	return &Service{store: store, clock: clock}
}

func (s *Service) SetMutator(mutator ports.QualityGateMutator) { s.mutator = mutator }

func (s *Service) requireMutator() error {
	if s.mutator == nil {
		return fmt.Errorf("%w: quality gate mutations are not configured", shared.ErrValidation)
	}
	return nil
}

func (s *Service) List(ctx context.Context, tenantID shared.ID) ([]qualitygate.Gate, error) {
	custom, err := s.store.List(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list quality gates: %w", err)
	}
	return append(qualitygate.BuiltIns(), custom...), nil
}

func (s *Service) Get(ctx context.Context, tenantID shared.ID, key string) (qualitygate.Gate, error) {
	if gate, ok := qualitygate.Resolve(key); ok {
		return gate, nil
	}
	gate, err := s.store.Get(ctx, tenantID, strings.TrimSpace(key))
	if err != nil {
		return qualitygate.Gate{}, fmt.Errorf("get quality gate: %w", err)
	}
	return gate, nil
}

func (s *Service) Create(ctx context.Context, actor string, tenantID shared.ID, gate qualitygate.Gate) (qualitygate.Gate, error) {
	if strings.TrimSpace(actor) == "" {
		return qualitygate.Gate{}, fmt.Errorf("%w: actor is required", shared.ErrValidation)
	}
	gate, err := gate.Normalize()
	if err != nil {
		return qualitygate.Gate{}, fmt.Errorf("%w: %v", shared.ErrValidation, err)
	}
	if _, builtIn := qualitygate.Resolve(gate.Key); builtIn {
		return qualitygate.Gate{}, fmt.Errorf("%w: quality gate key is reserved", shared.ErrValidation)
	}
	if err := s.requireMutator(); err != nil {
		return qualitygate.Gate{}, err
	}
	if err := s.mutator.CreateGate(ctx, tenantID, gate, ports.AuditEntry{Actor: actor, Action: "quality_gate.create", Target: gate.Key, Metadata: map[string]string{"gate": gate.Key}, At: s.clock.Now()}); err != nil {
		return qualitygate.Gate{}, fmt.Errorf("create quality gate: %w", err)
	}
	return gate, nil
}

func (s *Service) Update(ctx context.Context, actor string, tenantID shared.ID, key string, gate qualitygate.Gate) (qualitygate.Gate, error) {
	if strings.TrimSpace(actor) == "" {
		return qualitygate.Gate{}, fmt.Errorf("%w: actor is required", shared.ErrValidation)
	}
	gate.Key = strings.TrimSpace(key)
	gate, err := gate.Normalize()
	if err != nil {
		return qualitygate.Gate{}, fmt.Errorf("%w: %v", shared.ErrValidation, err)
	}
	if _, builtIn := qualitygate.Resolve(gate.Key); builtIn {
		return qualitygate.Gate{}, fmt.Errorf("%w: built-in quality gates cannot be edited", shared.ErrValidation)
	}
	if err := s.requireMutator(); err != nil {
		return qualitygate.Gate{}, err
	}
	if err := s.mutator.UpdateGate(ctx, tenantID, gate, ports.AuditEntry{Actor: actor, Action: "quality_gate.update", Target: gate.Key, Metadata: map[string]string{"gate": gate.Key}, At: s.clock.Now()}); err != nil {
		return qualitygate.Gate{}, fmt.Errorf("update quality gate: %w", err)
	}
	return gate, nil
}

func (s *Service) Delete(ctx context.Context, actor string, tenantID shared.ID, key string) error {
	if strings.TrimSpace(actor) == "" {
		return fmt.Errorf("%w: actor is required", shared.ErrValidation)
	}
	key = strings.TrimSpace(key)
	if _, builtIn := qualitygate.Resolve(key); builtIn {
		return fmt.Errorf("%w: built-in quality gates cannot be deleted", shared.ErrValidation)
	}
	if err := s.requireMutator(); err != nil {
		return err
	}
	if err := s.mutator.DeleteGate(ctx, tenantID, key, ports.AuditEntry{Actor: actor, Action: "quality_gate.delete", Target: key, Metadata: map[string]string{"gate": key}, At: s.clock.Now()}); err != nil {
		return fmt.Errorf("delete quality gate: %w", err)
	}
	return nil
}
