package whitespacezonerouting

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/httpapi"
	"timber-pest-remediation-ledger/internal/store"
)

func TestAcceptedWhitespaceZoneRemainsAddressable(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := cases.NewService(repository)
	defer service.Close()

	created, err := service.Create(ctx, cases.CreateCommand{RequestID: "create", ActorID: "survey", CaseID: "case-1", SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: []domain.TimberComponent{{ComponentID: "beam", LocationCode: "A1", ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 3, DamageExtentPercent: 12, MoisturePercent: 15, EvidenceDigest: "proof"}}})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.FreezeBaseline(ctx, created.CaseID, cases.CommandMeta{RequestID: "freeze", ActorID: "survey", ExpectedRevision: created.Revision})
	if err != nil {
		t.Fatal(err)
	}
	risk, err := service.AssessRisk(ctx, created.CaseID, cases.CommandMeta{RequestID: "risk", ActorID: "engineer", ExpectedRevision: frozen.Revision})
	if err != nil {
		t.Fatal(err)
	}
	planned, err := service.SubmitPlan(ctx, created.CaseID, cases.PlanCommand{CommandMeta: cases.CommandMeta{RequestID: "plan", ActorID: "planner", ExpectedRevision: risk.Revision}, Zones: []domain.ZonePlanInput{{ZoneID: " zone-a ", ComponentIDs: []string{"beam"}, Method: "注射", ApprovedParameters: map[string]float64{"dose": 100}, ProtectionConstraints: []string{"隔离"}, ResponsibleID: "field", AcceptanceThresholds: domain.AcceptanceThresholds{MinObservations: 1}}}})
	if err != nil {
		t.Fatalf("带空白 zone_id 的方案未被接受: %v", err)
	}
	approved, err := service.ApprovePlan(ctx, created.CaseID, cases.CommandMeta{RequestID: "approve", ActorID: "approver", ExpectedRevision: planned.Revision})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(httpapi.New(service).Handler())
	defer server.Close()
	body, err := json.Marshal(map[string]any{"request_id": "execute", "actor_id": "field", "expected_revision": approved.Revision, "execution": map[string]any{"actual_parameters": map[string]float64{"dose": 100}, "evidence_digest": "execution-proof"}})
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(server.URL+"/api/v1/cases/case-1/zones/"+url.PathEscape(" zone-a ")+"/executions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("TestAcceptedWhitespaceZoneRemainsAddressable: 已接受的 zone_id 经路径访问得到 HTTP %d", response.StatusCode)
	}
}
