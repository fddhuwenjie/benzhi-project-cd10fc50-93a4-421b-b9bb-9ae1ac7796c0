package coordinator_context_reuse_test

import (
	"context"
	"path/filepath"
	"testing"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

func TestFreshCommandDoesNotReuseCanceledWorkerContext(t *testing.T) {
	repository, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	service := cases.NewService(repository)
	defer service.Close()

	createCtx, cancelCreate := context.WithCancel(context.Background())
	cancelCreate()
	command := cases.CreateCommand{
		RequestID:    "create-canceled-context",
		ActorID:      "survey-lead",
		CaseID:       "case-context-owner",
		SiteName:     "古建一号",
		BuildingZone: "正殿",
		SurveyLeadID: "survey-lead",
		Components: []domain.TimberComponent{{
			ComponentID:         "beam-1",
			LocationCode:        "A-01",
			ComponentType:       "梁",
			HeritageGrade:       "I",
			PestClue:            "新鲜蛀孔",
			ActivityScore:       3,
			DamageExtentPercent: 12,
			MoisturePercent:     16,
			EvidenceDigest:      "evidence-create",
		}},
	}
	if _, err := service.Create(createCtx, command); err == nil {
		t.Fatal("已取消的首次创建请求意外成功")
	}

	command.RequestID = "create-fresh-context"
	created, err := service.Create(context.Background(), command)
	if err != nil {
		t.Fatalf("新请求复用了已取消的 worker context: %v", err)
	}
	if created.Status != domain.StatusDraft || created.Revision != 1 {
		t.Fatalf("创建结果异常: status=%s revision=%d", created.Status, created.Revision)
	}
}
