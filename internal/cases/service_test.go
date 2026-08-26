package cases

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

func TestIdempotencyReplayAndConflictSurviveRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ledger.db")
	repository, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repository)
	command := CreateCommand{RequestID: "create-1", ActorID: "survey", CaseID: "case-1", SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: []domain.TimberComponent{{ComponentID: "beam", LocationCode: "A1", ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 3, DamageExtentPercent: 12, MoisturePercent: 15, EvidenceDigest: "proof"}}}
	created, err := service.Create(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if created.Revision != 1 {
		t.Fatalf("创建 revision=%d", created.Revision)
	}
	frozen, err := service.FreezeBaseline(ctx, command.CaseID, CommandMeta{RequestID: "freeze-1", ActorID: "survey", ExpectedRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.FreezeBaseline(ctx, command.CaseID, CommandMeta{RequestID: "freeze-1", ActorID: "survey", ExpectedRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Revision != frozen.Revision || replayed.Status != domain.StatusBaselineFrozen {
		t.Fatalf("重放响应不一致")
	}
	_, err = service.FreezeBaseline(ctx, command.CaseID, CommandMeta{RequestID: "freeze-1", ActorID: "other", ExpectedRevision: 1})
	var business *domain.BusinessError
	if !errors.As(err, &business) || business.Code != "idempotency_conflict" {
		t.Fatalf("预期 idempotency_conflict，得到 %v", err)
	}
	service.Close()
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	repository, err = store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service = NewService(repository)
	defer service.Close()
	replayed, err = service.FreezeBaseline(ctx, command.CaseID, CommandMeta{RequestID: "freeze-1", ActorID: "survey", ExpectedRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Revision != 2 {
		t.Fatalf("跨重启重放 revision=%d", replayed.Revision)
	}
}

func TestComponentRevisionBatchCommitsOnce(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, filepath.Join(t.TempDir(), "revision.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := NewService(repository)
	defer service.Close()
	components := []domain.TimberComponent{
		{ComponentID: "a", LocationCode: "A", ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 2, DamageExtentPercent: 10, MoisturePercent: 15, EvidenceDigest: "a"},
		{ComponentID: "b", LocationCode: "B", ComponentType: "柱", HeritageGrade: "II", PestClue: "虫粪", ActivityScore: 3, DamageExtentPercent: 20, MoisturePercent: 16, EvidenceDigest: "b"},
	}
	created, err := service.Create(ctx, CreateCommand{RequestID: "create", ActorID: "survey", CaseID: "revision", SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: components})
	if err != nil {
		t.Fatal(err)
	}
	replacement := components[0]
	replacement.MoisturePercent = 18
	third := domain.TimberComponent{ComponentID: "c", LocationCode: "C", ComponentType: "檩", HeritageGrade: "III", PestClue: "活虫", ActivityScore: 4, DamageExtentPercent: 30, MoisturePercent: 17, EvidenceDigest: "c"}
	command := ComponentRevisionCommand{CommandMeta: CommandMeta{RequestID: "revise", ActorID: "engineer", ExpectedRevision: created.Revision}, Operations: []domain.ComponentRevisionOperation{{Operation: "replace", Component: &replacement}, {Operation: "remove", ComponentID: "b"}, {Operation: "add", Component: &third}}}
	revised, err := service.ReviseBaselineComponents(ctx, created.CaseID, command)
	if err != nil {
		t.Fatal(err)
	}
	if revised.Revision != 2 || len(revised.Components) != 2 {
		t.Fatalf("批次 revision 或构件数错误: %#v", revised)
	}
	replayed, err := service.ReviseBaselineComponents(ctx, created.CaseID, command)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Revision != revised.Revision {
		t.Fatal("幂等重放结果不一致")
	}
	events, err := service.Timeline(ctx, created.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].EventType != "batch_baseline_components_revised" {
		t.Fatalf("批次审计事件错误: %#v", events)
	}
	if err := repository.VerifyIntegrity(ctx); err != nil {
		t.Fatalf("规范化构件与聚合不一致: %v", err)
	}
}
