package archivemanifeststartup

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"

	_ "modernc.org/sqlite"
)

func TestStartupRejectsCorruptedArchiveManifest(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ledger.db")
	repository, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	service := cases.NewService(repository)
	archiveCase(t, ctx, service)
	service.Close()
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE archives SET manifest_json=? WHERE case_id=?`, []byte(`{"tampered":true}`), "case-1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(ctx, path)
	if err == nil {
		reopened.Close()
		t.Fatal("TestStartupRejectsCorruptedArchiveManifest: 归档清单字节与持久化摘要不符时仍允许启动")
	}
}

func archiveCase(t *testing.T, ctx context.Context, service *cases.Service) {
	t.Helper()
	c, err := service.Create(ctx, cases.CreateCommand{RequestID: "create", ActorID: "survey", CaseID: "case-1", SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: []domain.TimberComponent{{ComponentID: "beam", LocationCode: "A1", ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 3, DamageExtentPercent: 12, MoisturePercent: 15, EvidenceDigest: "proof"}}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.FreezeBaseline(ctx, c.CaseID, cases.CommandMeta{RequestID: "freeze", ActorID: "survey", ExpectedRevision: c.Revision})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.AssessRisk(ctx, c.CaseID, cases.CommandMeta{RequestID: "risk", ActorID: "engineer", ExpectedRevision: c.Revision})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.SubmitPlan(ctx, c.CaseID, cases.PlanCommand{CommandMeta: cases.CommandMeta{RequestID: "plan", ActorID: "planner", ExpectedRevision: c.Revision}, Zones: []domain.ZonePlanInput{{ZoneID: "zone-a", ComponentIDs: []string{"beam"}, Method: "注射", ApprovedParameters: map[string]float64{"dose": 100}, ProtectionConstraints: []string{"隔离"}, ResponsibleID: "field", AcceptanceThresholds: domain.AcceptanceThresholds{MinObservations: 1}}}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.ApprovePlan(ctx, c.CaseID, cases.CommandMeta{RequestID: "approve", ActorID: "approver", ExpectedRevision: c.Revision})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.RecordExecution(ctx, c.CaseID, "zone-a", cases.ExecutionCommand{CommandMeta: cases.CommandMeta{RequestID: "execute", ActorID: "field", ExpectedRevision: c.Revision}, Execution: domain.ExecutionInput{ActualParameters: map[string]float64{"dose": 100}, EvidenceDigest: "execution"}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.AddObservation(ctx, c.CaseID, "zone-a", cases.ObservationCommand{CommandMeta: cases.CommandMeta{RequestID: "observe", ActorID: "monitor", ExpectedRevision: c.Revision}, Observation: domain.ObservationInput{ObservationID: "observation-1", Method: "trap", VisualFinding: "none", EvidenceDigest: "monitoring"}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.EvaluateZone(ctx, c.CaseID, "zone-a", cases.CommandMeta{RequestID: "evaluate", ActorID: "engineer", ExpectedRevision: c.Revision})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Review(ctx, c.CaseID, cases.ReviewCommand{CommandMeta: cases.CommandMeta{RequestID: "review", ActorID: "reviewer", ExpectedRevision: c.Revision}, Decision: "pass", Findings: "证据完整", EvidenceComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != domain.StatusArchived {
		t.Fatalf("未进入归档状态: %s", c.Status)
	}
}
