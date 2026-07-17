package memory

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/KKloudTarus/synapse-ce/internal/domain/qualitygate"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
)

// QualityGateStore is a goroutine-safe in-memory custom quality-gate store.
type QualityGateStore struct {
	mu   sync.RWMutex
	data map[string]qualitygate.Gate
}

func NewQualityGateStore() *QualityGateStore {
	return &QualityGateStore{data: map[string]qualitygate.Gate{}}
}

var _ ports.QualityGateStore = (*QualityGateStore)(nil)

func qualityGateStoreKey(tenantID shared.ID, key string) string {
	return tenantID.String() + "\x00" + strings.TrimSpace(key)
}

func cloneQualityGate(in qualitygate.Gate) qualitygate.Gate {
	in.Conditions = append([]qualitygate.Condition(nil), in.Conditions...)
	return in
}

func (s *QualityGateStore) Create(_ context.Context, tenantID shared.ID, gate qualitygate.Gate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := qualityGateStoreKey(tenantID, gate.Key)
	if _, found := s.data[key]; found {
		return shared.ErrConflict
	}
	s.data[key] = cloneQualityGate(gate)
	return nil
}

func (s *QualityGateStore) List(_ context.Context, tenantID shared.ID) ([]qualitygate.Gate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]qualitygate.Gate, 0, len(s.data))
	prefix := tenantID.String() + "\x00"
	for key, gate := range s.data {
		if tenantID.IsZero() || strings.HasPrefix(key, prefix) {
			out = append(out, cloneQualityGate(gate))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *QualityGateStore) Get(_ context.Context, tenantID shared.ID, key string) (qualitygate.Gate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	gate, found := s.data[qualityGateStoreKey(tenantID, key)]
	if !found {
		return qualitygate.Gate{}, shared.ErrNotFound
	}
	return cloneQualityGate(gate), nil
}

func (s *QualityGateStore) Update(_ context.Context, tenantID shared.ID, gate qualitygate.Gate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := qualityGateStoreKey(tenantID, gate.Key)
	if _, found := s.data[key]; !found {
		return shared.ErrNotFound
	}
	s.data[key] = cloneQualityGate(gate)
	return nil
}

func (s *QualityGateStore) Delete(_ context.Context, tenantID shared.ID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key = qualityGateStoreKey(tenantID, key)
	if _, found := s.data[key]; !found {
		return shared.ErrNotFound
	}
	delete(s.data, key)
	return nil
}

func (s *QualityGateStore) DeleteIfUnassigned(ctx context.Context, tenantID shared.ID, key string) error {
	return s.Delete(ctx, tenantID, key)
}
