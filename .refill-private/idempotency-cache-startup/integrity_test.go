package idempotencycachestartup

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

func TestStartupRejectsCorruptedIdempotencyResponse(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ledger.db")
	repository, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	service := cases.NewService(repository)
	created, err := service.Create(ctx, cases.CreateCommand{RequestID: "create", ActorID: "survey", CaseID: "case-1", SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: []domain.TimberComponent{{ComponentID: "beam", LocationCode: "A1", ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 3, DamageExtentPercent: 12, MoisturePercent: 15, EvidenceDigest: "proof"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.FreezeBaseline(ctx, created.CaseID, cases.CommandMeta{RequestID: "freeze", ActorID: "survey", ExpectedRevision: created.Revision}); err != nil {
		t.Fatal(err)
	}
	service.Close()
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE idempotency SET response_json=? WHERE case_id=? AND request_id=?`, []byte(`{"revision":`), "case-1", "freeze"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(ctx, path)
	if err == nil {
		reopened.Close()
		t.Fatal("TestStartupRejectsCorruptedIdempotencyResponse: 无法解码的幂等响应缓存仍允许启动")
	}
}
