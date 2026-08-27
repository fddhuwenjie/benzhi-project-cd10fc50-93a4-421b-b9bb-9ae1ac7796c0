package timelinecacheinvalidation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/httpapi"
	"timber-pest-remediation-ledger/internal/store"
)

func TestTimelineCacheInvalidatedAfterMutation(t *testing.T) {
	repository, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := cases.NewService(repository)
	defer service.Close()
	server := httptest.NewServer(httpapi.New(service).Handler())
	defer server.Close()

	create := `{"request_id":"create-1","actor_id":"survey","case_id":"case-1","site_name":"site","building_zone":"hall","survey_lead_id":"survey","components":[{"component_id":"beam-1","location_code":"A1","component_type":"beam","heritage_grade":"I","pest_clue":"holes","activity_score":3,"damage_extent_percent":12,"moisture_percent":15,"evidence_digest":"proof"}]}`
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/cases", create, http.StatusCreated)
	if got := timelineLength(t, server.URL+"/api/v1/cases/case-1/timeline"); got != 1 {
		t.Fatalf("initial timeline has %d events, want 1", got)
	}

	freeze := `{"request_id":"freeze-1","actor_id":"survey","expected_revision":1}`
	requestJSON(t, http.MethodPost, server.URL+"/api/v1/cases/case-1/baseline/freeze", freeze, http.StatusOK)
	if got := timelineLength(t, server.URL+"/api/v1/cases/case-1/timeline"); got != 2 {
		t.Fatalf("timeline after mutation has %d events, want 2", got)
	}
}

func requestJSON(t *testing.T, method, url, body string, wantStatus int) {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != wantStatus {
		t.Fatalf("%s %s returned %d, want %d", method, url, response.StatusCode, wantStatus)
	}
}

func timelineLength(t *testing.T, url string) int {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %d", url, response.StatusCode)
	}
	var envelope struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(fmt.Errorf("decode timeline: %w", err))
	}
	return len(envelope.Data)
}
