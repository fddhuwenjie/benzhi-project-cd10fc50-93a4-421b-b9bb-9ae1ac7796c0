package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/store"
)

func TestStrictJSONAndContentType(t *testing.T) {
	repository, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := cases.NewService(repository)
	defer service.Close()
	server := httptest.NewServer(New(service).Handler())
	defer server.Close()
	response, err := http.Post(server.URL+"/api/v1/cases", "text/plain", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("错误 Content-Type 状态=%d", response.StatusCode)
	}
	response.Body.Close()
	body := `{"request_id":"r1","actor_id":"survey","case_id":"c1","site_name":"古建","building_zone":"正殿","survey_lead_id":"survey","components":[],"unknown":true}`
	response, err = http.Post(server.URL+"/api/v1/cases", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("未知字段状态=%d", response.StatusCode)
	}
}
