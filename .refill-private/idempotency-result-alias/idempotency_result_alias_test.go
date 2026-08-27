package idempotencyresultalias_test

import (
	"context"
	"path/filepath"
	"testing"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

func TestIdempotencyReplayIsolatedFromCallerMutation(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := cases.NewService(repository)
	defer service.Close()

	created, err := service.Create(ctx, cases.CreateCommand{
		RequestID: "create-alias-case", ActorID: "survey", CaseID: "alias-case",
		SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey",
		Components: []domain.TimberComponent{{
			ComponentID: "beam-1", LocationCode: "A1", ComponentType: "梁",
			HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 3,
			DamageExtentPercent: 12, MoisturePercent: 15, EvidenceDigest: "proof-1",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	meta := cases.CommandMeta{RequestID: "freeze-alias", ActorID: "survey", ExpectedRevision: created.Revision}
	first, err := service.FreezeBaseline(ctx, created.CaseID, meta)
	if err != nil {
		t.Fatal(err)
	}

	first.Status = domain.StatusArchived
	first.Components[0].EvidenceDigest = "caller-overwrite"

	persisted, err := service.Get(ctx, created.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Status != domain.StatusBaselineFrozen || persisted.Components[0].EvidenceDigest != "proof-1" {
		t.Fatalf("持久化快照意外变化: status=%s evidence=%s", persisted.Status, persisted.Components[0].EvidenceDigest)
	}

	replayed, err := service.FreezeBaseline(ctx, created.CaseID, meta)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Status != domain.StatusBaselineFrozen || replayed.Components[0].EvidenceDigest != "proof-1" {
		t.Fatalf("幂等重放复用了被调用方污染的缓存结果: status=%s evidence=%s", replayed.Status, replayed.Components[0].EvidenceDigest)
	}
}
