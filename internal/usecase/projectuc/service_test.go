package projectuc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KKloudTarus/synapse-ce/internal/domain/finding"
	"github.com/KKloudTarus/synapse-ce/internal/domain/measure"
	"github.com/KKloudTarus/synapse-ce/internal/domain/project"
	"github.com/KKloudTarus/synapse-ce/internal/domain/qualitygate"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/infrastructure/persistence/memory"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
	qualitygatesuc "github.com/KKloudTarus/synapse-ce/internal/usecase/qualitygates"
	scauc "github.com/KKloudTarus/synapse-ce/internal/usecase/sca"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type fixedIDs struct{}

func (fixedIDs) NewID() shared.ID { return "p1" }

type captureAudit struct{ entries []ports.AuditEntry }

func (a *captureAudit) Record(_ context.Context, e ports.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func TestServiceCRUDAndAudit(t *testing.T) {
	ctx := context.Background()
	audit := &captureAudit{}
	svc := NewService(memory.NewProjectRepository(), memory.NewEngagementRepository(), fixedClock{time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)}, fixedIDs{}, audit, true)
	p, err := svc.Create(ctx, CreateInput{TenantID: "tenant-a", CreatedBy: "alice", Name: "Project", Key: "project", SourceBinding: project.SourceBinding{Kind: project.SourceLocal, Value: "/repo"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Audit.CreatedBy != "alice" || len(audit.entries) != 1 || audit.entries[0].Action != "project.create" {
		t.Fatalf("create audit/owner: p=%+v audit=%+v", p, audit.entries)
	}
	if _, err := svc.Get(ctx, "tenant-b", "project"); !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("cross-tenant=%v, want not found", err)
	}
	list, err := svc.List(ctx, "tenant-a")
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if err := svc.Delete(ctx, "alice", "tenant-a", "project"); err != nil {
		t.Fatal(err)
	}
	if len(audit.entries) != 2 || audit.entries[1].Action != "project.delete" {
		t.Fatalf("delete audit=%+v", audit.entries)
	}
}

func TestServiceRequiresActor(t *testing.T) {
	svc := NewService(memory.NewProjectRepository(), memory.NewEngagementRepository(), fixedClock{}, fixedIDs{}, &captureAudit{}, true)
	if _, err := svc.Create(context.Background(), CreateInput{Name: "P", Key: "p", SourceBinding: project.SourceBinding{Kind: project.SourceLocal, Value: "/repo"}}); !errors.Is(err, shared.ErrValidation) {
		t.Fatalf("got %v, want validation", err)
	}
}

func TestServiceRejectsLocalSourceOutsideDevelopment(t *testing.T) {
	svc := NewService(memory.NewProjectRepository(), memory.NewEngagementRepository(), fixedClock{}, fixedIDs{}, &captureAudit{}, false)
	_, err := svc.Create(context.Background(), CreateInput{
		TenantID: "tenant-a", CreatedBy: "alice", Name: "Project", Key: "project",
		SourceBinding: project.SourceBinding{Kind: project.SourceLocal, Value: "/repo"},
	})
	if !errors.Is(err, shared.ErrValidation) {
		t.Fatalf("got %v, want validation", err)
	}
}

func TestRecordProjectAnalysisUsesAssignedGate(t *testing.T) {
	ctx := context.Background()
	projects := memory.NewProjectRepository()
	engagements := memory.NewEngagementRepository()
	analyses := memory.NewProjectAnalysisStore()
	svc := NewService(projects, engagements, fixedClock{}, fixedIDs{}, &captureAudit{}, true)
	svc.SetAnalysisStore(analyses)
	svc.SetQualityGates(qualitygatesuc.NewService(memory.NewQualityGateStore(), &captureAudit{}, fixedClock{}))
	if _, err := svc.gates.Create(ctx, "alice", "tenant", qualitygate.Gate{Key: "relaxed", Name: "Relaxed", Conditions: []qualitygate.Condition{{Metric: qualitygate.MetricNewHigh, Op: qualitygate.OpLE, Threshold: 1}}}); err != nil {
		t.Fatal(err)
	}
	p, err := svc.Create(ctx, CreateInput{TenantID: "tenant", CreatedBy: "alice", Name: "Project", Key: "project", GateID: "relaxed", SourceBinding: project.SourceBinding{Kind: project.SourceLocal, Value: "/repo"}})
	if err != nil {
		t.Fatal(err)
	}
	e, err := engagements.GetByProjectID(ctx, p.TenantID, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordProjectAnalysis(ctx, e.ID, "job-1", time.Unix(1, 0), &scauc.ScanResult{Findings: []finding.Finding{{ID: "high", DedupKey: "high", Kind: finding.KindSCA, Severity: shared.SeverityHigh, Status: finding.StatusOpen}}}); err != nil {
		t.Fatal(err)
	}
	list, _, err := analyses.List(ctx, p.TenantID, p.ID, 1, time.Time{}, "")
	if err != nil || len(list) != 1 || !list[0].Gate.Passed || len(list[0].Gate.Results) != 1 {
		t.Fatalf("analysis=%+v err=%v", list, err)
	}
}

func TestRecordProjectAnalysisPersistsLineCoverage(t *testing.T) {
	ctx := context.Background()
	projects := memory.NewProjectRepository()
	engagements := memory.NewEngagementRepository()
	analyses := memory.NewProjectAnalysisStore()
	svc := NewService(projects, engagements, fixedClock{}, fixedIDs{}, &captureAudit{}, true)
	svc.SetAnalysisStore(analyses)
	p, err := svc.Create(ctx, CreateInput{TenantID: "tenant", CreatedBy: "alice", Name: "Project", Key: "project", SourceBinding: project.SourceBinding{Kind: project.SourceLocal, Value: "/repo"}})
	if err != nil {
		t.Fatal(err)
	}
	e, err := engagements.GetByProjectID(ctx, p.TenantID, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	coverage := &measure.CoverageReport{Files: []measure.FileCoverage{{File: "a.go", CoveredLines: 1, TotalLines: 2}}, CoveredLines: 1, TotalLines: 2}
	if err := svc.RecordProjectAnalysis(ctx, e.ID, "job-1", time.Unix(1, 0), &scauc.ScanResult{LineCoverage: coverage}); err != nil {
		t.Fatal(err)
	}
	list, _, err := analyses.List(ctx, p.TenantID, p.ID, 1, time.Time{}, "")
	if err != nil || len(list) != 1 || list[0].Coverage == nil || list[0].Coverage.Percent() != 50 || list[0].Measures[qualitygate.MetricCoveragePct] != 50 {
		t.Fatalf("analysis=%+v err=%v", list, err)
	}
}

func TestRecordProjectAnalysisHydratesCurrentTriageOnly(t *testing.T) {
	ctx := context.Background()
	projects := memory.NewProjectRepository()
	engagements := memory.NewEngagementRepository()
	analyses := memory.NewProjectAnalysisStore()
	findings := memory.NewFindingRepository()
	svc := NewService(projects, engagements, fixedClock{}, fixedIDs{}, &captureAudit{}, true)
	svc.SetAnalysisStore(analyses)
	svc.SetFindingRepository(findings)
	p, err := svc.Create(ctx, CreateInput{TenantID: "tenant-a", CreatedBy: "alice", Name: "Project", Key: "project", SourceBinding: project.SourceBinding{Kind: project.SourceLocal, Value: "/repo"}})
	if err != nil {
		t.Fatal(err)
	}
	e, err := engagements.GetByProjectID(ctx, p.TenantID, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	persisted := []finding.Finding{
		{ID: "current-id", EngagementID: e.ID, DedupKey: "current", Status: finding.StatusFalsePos, Severity: shared.SeverityHigh, Kind: finding.KindSCA},
		{ID: "stale-id", EngagementID: e.ID, DedupKey: "stale", Status: finding.StatusRemediated, Severity: shared.SeverityCritical, Kind: finding.KindSCA},
	}
	if err := findings.Upsert(ctx, persisted); err != nil {
		t.Fatal(err)
	}
	result := &scauc.ScanResult{Findings: []finding.Finding{{ID: "new-id", EngagementID: e.ID, DedupKey: "current", Status: finding.StatusOpen, Severity: shared.SeverityHigh, Kind: finding.KindSCA}}}
	if err := svc.RecordProjectAnalysis(ctx, e.ID, "job-1", time.Unix(1, 0), result); err != nil {
		t.Fatal(err)
	}
	list, _, err := analyses.List(ctx, p.TenantID, p.ID, 1, time.Time{}, "")
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if got := list[0].Issues.ByStatus[string(finding.StatusFalsePos)]; got != 1 {
		t.Fatalf("false-positive count=%d, want 1", got)
	}
	if list[0].Issues.Total != 1 || len(list[0].InternalIssues) != 1 || list[0].InternalIssues[0].Key != "current" {
		t.Fatalf("stale finding leaked into snapshot: %+v", list[0])
	}
}
