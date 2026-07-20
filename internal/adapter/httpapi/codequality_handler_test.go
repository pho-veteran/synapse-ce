package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KKloudTarus/synapse-ce/internal/domain/finding"
	"github.com/KKloudTarus/synapse-ce/internal/domain/shared"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/codequality"
	"github.com/KKloudTarus/synapse-ce/internal/usecase/ports"
	scauc "github.com/KKloudTarus/synapse-ce/internal/usecase/sca"
)

type codeQualityFake struct{ calls int }

func (s *codeQualityFake) BuildReport(context.Context, string) (codequality.Report, error) {
	s.calls++
	return codequality.Report{}, nil
}

type scanResultsFake struct {
	data []byte
	err  error
}

func (s scanResultsFake) SaveResult(context.Context, shared.ID, []byte) error { return nil }
func (s scanResultsFake) LatestResult(context.Context, shared.ID) ([]byte, error) {
	return s.data, s.err
}

func newCodeQualityRouter(t *testing.T, results ports.ScanResultStore, analyzer *codeQualityFake) *Router {
	t.Helper()
	rt, _, _ := newEngRouter(t)
	rt.sca = scauc.NewService(nil, nil, nil, results, nil, nil, nil, nil, ports.Provenance{}, fixedClock{}, &fakeAudit{}, shared.SeverityInfo, 0, nil, nil, nil, nil, nil, nil, nil)
	rt.SetCodeQuality(analyzer)
	return rt
}

func codeQualityCall(rt *Router) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/engagements/eng-1/code-quality", nil)
	req.SetPathValue("id", "eng-1")
	rec := httptest.NewRecorder()
	rt.codeQualityReport(rec, req)
	return rec
}

func TestCodeQualityReportReturnsStoredScanResult(t *testing.T) {
	stored := codequality.Report{Findings: []finding.Finding{{Title: "Stored finding"}}}
	data, err := json.Marshal(struct {
		CodeQuality *codequality.Report `json:"code_quality"`
	}{CodeQuality: &stored})
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &codeQualityFake{}
	rec := codeQualityCall(newCodeQualityRouter(t, scanResultsFake{data: data}, analyzer))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got codeQualityReportView
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.Report == nil || len(got.Report.Findings) != 1 || got.Report.Findings[0].Title != "Stored finding" {
		t.Fatalf("response = %+v", got)
	}
	if analyzer.calls != 0 {
		t.Fatalf("BuildReport calls = %d, want 0", analyzer.calls)
	}
}

func TestCodeQualityReportWithoutStoredReportIsUnavailable(t *testing.T) {
	analyzer := &codeQualityFake{}
	rec := codeQualityCall(newCodeQualityRouter(t, scanResultsFake{data: []byte(`{}`)}, analyzer))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got codeQualityReportView
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Available || got.Reason != codeQualityUnavailable {
		t.Fatalf("response = %+v", got)
	}
	if analyzer.calls != 0 {
		t.Fatalf("BuildReport calls = %d, want 0", analyzer.calls)
	}
}

func TestCodeQualityReportWithoutScanIsUnavailable(t *testing.T) {
	analyzer := &codeQualityFake{}
	rec := codeQualityCall(newCodeQualityRouter(t, scanResultsFake{err: shared.ErrNotFound}, analyzer))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got codeQualityReportView
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Available || got.Reason != codeQualityUnavailable {
		t.Fatalf("response = %+v", got)
	}
	if analyzer.calls != 0 {
		t.Fatalf("BuildReport calls = %d, want 0", analyzer.calls)
	}
}

func TestCodeQualityReportRejectsInvalidStoredResult(t *testing.T) {
	rec := codeQualityCall(newCodeQualityRouter(t, scanResultsFake{data: []byte("not json")}, &codeQualityFake{}))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestCodeQualityReportSurfacesResultStoreError(t *testing.T) {
	rec := codeQualityCall(newCodeQualityRouter(t, scanResultsFake{err: errors.New("store unavailable")}, &codeQualityFake{}))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
