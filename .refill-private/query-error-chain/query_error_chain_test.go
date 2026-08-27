package queryerrorchain_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/httpapi"
	"timber-pest-remediation-ledger/internal/store"
)

func TestMissingQueryErrorsPreserveNotFound(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	service := cases.NewService(repository)
	t.Cleanup(service.Close)
	handler := httpapi.New(service).Handler()

	paths := map[string]string{
		"get":                "/api/v1/cases/missing",
		"monitoring_summary": "/api/v1/cases/missing/monitoring/summary",
		"timeline":           "/api/v1/cases/missing/timeline",
		"archive":            "/api/v1/cases/missing/archive",
		"archive_verify":     "/api/v1/cases/missing/archive/verify",
		"archive_report":     "/api/v1/cases/missing/archive/verification-report",
	}
	for name, path := range paths {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
