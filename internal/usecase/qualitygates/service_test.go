package qualitygates

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KKloudTarus/synapse-ce/internal/domain/qualitygate"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/infrastructure/persistence/memory"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
)

type testClock struct{}

func (testClock) Now() time.Time { return time.Unix(1, 0) }

type testAudit struct{}

func (testAudit) Record(context.Context, ports.AuditEntry) error { return nil }

func TestServiceManagesOnlyCustomGates(t *testing.T) {
	svc := NewService(memory.NewQualityGateStore(), testAudit{}, testClock{})
	gate, err := svc.Create(context.Background(), "alice", "tenant", qualitygate.Gate{
		Key: "release", Name: "Release", Conditions: []qualitygate.Condition{{Metric: qualitygate.MetricNewHigh, Op: qualitygate.OpLE, Threshold: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gate.BuiltIn || gate.Key != "release" {
		t.Fatalf("gate=%+v", gate)
	}
	all, err := svc.List(context.Background(), "tenant")
	if err != nil || len(all) != 2 || !all[0].BuiltIn {
		t.Fatalf("gates=%+v err=%v", all, err)
	}
	if _, err := svc.Update(context.Background(), "alice", "tenant", qualitygate.DefaultKey, qualitygate.Gate{Name: "nope"}); !errors.Is(err, shared.ErrValidation) {
		t.Fatalf("built-in update=%v, want validation", err)
	}
}
