package archive_cache_alias_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

func TestArchiveCacheIsolatedFromCallerMutation(t *testing.T) {
	ctx := context.Background()
	repository, err := store.Open(ctx, filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := cases.NewService(repository)
	defer service.Close()

	current, err := service.Create(ctx, cases.CreateCommand{
		RequestID: "create", ActorID: "survey", CaseID: "cache-case",
		SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey",
		Components: []domain.TimberComponent{{
			ComponentID: "beam", LocationCode: "A1", ComponentType: "梁",
			HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 3,
			DamageExtentPercent: 12, MoisturePercent: 15, EvidenceDigest: "survey-proof",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	nextMeta := func(requestID, actorID string) cases.CommandMeta {
		return cases.CommandMeta{RequestID: requestID, ActorID: actorID, ExpectedRevision: current.Revision}
	}
	current, err = service.FreezeBaseline(ctx, current.CaseID, nextMeta("freeze", "survey"))
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.AssessRisk(ctx, current.CaseID, nextMeta("risk", "engineer"))
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.SubmitPlan(ctx, current.CaseID, cases.PlanCommand{
		CommandMeta: nextMeta("plan", "planner"),
		Zones: []domain.ZonePlanInput{{
			ZoneID: "zone-a", ComponentIDs: []string{"beam"}, Method: "注射",
			ApprovedParameters:    map[string]float64{"dose": 100},
			ProtectionConstraints: []string{"隔离彩画"}, ResponsibleID: "field",
			AcceptanceThresholds: domain.AcceptanceThresholds{MaxActivityCount: 0, MaxAcousticScore: 0.2, MinObservations: 1},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.ApprovePlan(ctx, current.CaseID, nextMeta("approve", "approver"))
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.RecordExecution(ctx, current.CaseID, "zone-a", cases.ExecutionCommand{
		CommandMeta: nextMeta("execute", "field"),
		Execution:   domain.ExecutionInput{ActualParameters: map[string]float64{"dose": 100}, EvidenceDigest: "execution-proof"},
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.AddObservation(ctx, current.CaseID, "zone-a", cases.ObservationCommand{
		CommandMeta: nextMeta("observe", "monitor"),
		Observation: domain.ObservationInput{ObservationID: "obs-1", Method: "trap", EvidenceDigest: "monitor-proof"},
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.EvaluateZone(ctx, current.CaseID, "zone-a", nextMeta("evaluate", "engineer"))
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.Review(ctx, current.CaseID, cases.ReviewCommand{
		CommandMeta: nextMeta("review", "reviewer"), Decision: "pass",
		Findings: "证据完整，处置闭合", EvidenceComplete: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	record, err := service.Archive(ctx, current.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Manifest) == 0 {
		t.Fatal("归档清单为空")
	}
	originalManifest := bytes.Clone(record.Manifest)
	record.Manifest[0] ^= 0xff

	secondRecord, err := service.Archive(ctx, current.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(secondRecord.Manifest, originalManifest) {
		t.Error("再次读取归档时返回了被调用方修改的 Manifest")
	}

	report, err := service.ArchiveVerificationReport(ctx, current.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid {
		for _, check := range report.Checks {
			if check.CheckCode == "manifest_bytes" && check.Status == "fail" {
				t.Fatal("调用方修改读取结果后污染了缓存，manifest_bytes 校验失败")
			}
		}
		t.Fatal("调用方修改读取结果后导致归档校验报告无效")
	}
}
