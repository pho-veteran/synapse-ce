package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KKloudTarus/synapse-ce/internal/domain/project"
	"github.com/KKloudTarus/synapse-ce/internal/domain/qualitygate"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
)

type mutatorAudit struct {
	entries []ports.AuditEntry
	err     error
}

func (a *mutatorAudit) Record(_ context.Context, entry ports.AuditEntry) error {
	if a.err != nil {
		return a.err
	}
	a.entries = append(a.entries, entry)
	return nil
}

func TestQualityGateMutatorLeavesStateUnchangedWhenAuditFails(t *testing.T) {
	ctx := context.Background()
	projects := NewProjectRepository()
	gates := NewQualityGateStore()
	audit := &mutatorAudit{err: errors.New("audit unavailable")}
	mutator := NewQualityGateMutator(gates, projects, audit)
	gate := qualitygate.Gate{Key: "release", Name: "Release", Conditions: []qualitygate.Condition{{Metric: qualitygate.MetricNewHigh, Op: qualitygate.OpLE, Threshold: 1}}}
	if err := mutator.CreateGate(ctx, "tenant", gate, ports.AuditEntry{}); err == nil {
		t.Fatal("create succeeded despite audit failure")
	}
	if _, err := gates.Get(ctx, "tenant", gate.Key); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("gate after failed create=%v, want not found", err)
	}

	p, err := project.New("project", "tenant", "Project", "project", project.SourceBinding{Kind: project.SourceLocal, Value: "/repo"}, nil, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := projects.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := mutator.AssignProjectGate(ctx, "tenant", p.Key, "", ports.AuditEntry{}); err == nil {
		t.Fatal("assignment succeeded despite audit failure")
	}
	stored, err := projects.GetByKey(ctx, "tenant", p.Key)
	if err != nil || stored.GateID != "" {
		t.Fatalf("project after failed assignment=%+v err=%v", stored, err)
	}
}

func TestQualityGateMutatorNeverDeletesAssignedGate(t *testing.T) {
	ctx := context.Background()
	projects := NewProjectRepository()
	gates := NewQualityGateStore()
	audit := &mutatorAudit{}
	mutator := NewQualityGateMutator(gates, projects, audit)
	gate := qualitygate.Gate{Key: "release", Name: "Release", Conditions: []qualitygate.Condition{{Metric: qualitygate.MetricNewHigh, Op: qualitygate.OpLE, Threshold: 1}}}
	if err := mutator.CreateGate(ctx, "tenant", gate, ports.AuditEntry{Action: "create"}); err != nil {
		t.Fatal(err)
	}
	p, err := project.New("project", "tenant", "Project", "project", project.SourceBinding{Kind: project.SourceLocal, Value: "/repo"}, nil, gate.Key, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := mutator.CreateProjectWithGate(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := mutator.DeleteGate(ctx, "tenant", gate.Key, ports.AuditEntry{Action: "delete"}); !errors.Is(err, shared.ErrConflict) {
		t.Fatalf("delete assigned gate=%v, want conflict", err)
	}
	if _, err := gates.Get(ctx, "tenant", gate.Key); err != nil {
		t.Fatalf("assigned gate missing: %v", err)
	}
	if len(audit.entries) != 1 {
		t.Fatalf("audit entries=%d, want 1", len(audit.entries))
	}
}

func TestQualityGateMutatorCreateProjectWithGateRejectsMissingCustomGate(t *testing.T) {
	ctx := context.Background()
	projects := NewProjectRepository()
	mutator := NewQualityGateMutator(NewQualityGateStore(), projects, &mutatorAudit{})
	p, err := project.New("project", "tenant", "Project", "project", project.SourceBinding{Kind: project.SourceLocal, Value: "/repo"}, nil, "release", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := mutator.CreateProjectWithGate(ctx, p); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("create with missing gate=%v, want not found", err)
	}
	if _, err := projects.GetByKey(ctx, "tenant", p.Key); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("project after failed create=%v, want not found", err)
	}
}

func TestQualityGateMutatorCreateProjectWithGateAcceptsBuiltIn(t *testing.T) {
	ctx := context.Background()
	projects := NewProjectRepository()
	mutator := NewQualityGateMutator(NewQualityGateStore(), projects, &mutatorAudit{})
	p, err := project.New("project", "tenant", "Project", "project", project.SourceBinding{Kind: project.SourceLocal, Value: "/repo"}, nil, qualitygate.DefaultKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := mutator.CreateProjectWithGate(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.GetByKey(ctx, "tenant", p.Key); err != nil {
		t.Fatalf("built-in project missing: %v", err)
	}
}
