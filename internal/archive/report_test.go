package archive

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

type reportFixture struct {
	c       *domain.RemediationCase
	events  []domain.AuditEvent
	archive *store.ArchiveRecord
	facts   *store.NormalizedFacts
}

func (f reportFixture) GetCase(context.Context, string) (*domain.RemediationCase, error) {
	return f.c, nil
}
func (f reportFixture) ListEvents(context.Context, string) ([]domain.AuditEvent, error) {
	return f.events, nil
}
func (f reportFixture) GetArchive(context.Context, string) (*store.ArchiveRecord, error) {
	return f.archive, nil
}
func (f reportFixture) ReadNormalizedFacts(context.Context, string) (*store.NormalizedFacts, error) {
	return f.facts, nil
}

func TestVerificationReportChecksAllLayers(t *testing.T) {
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	component := domain.TimberComponent{ComponentID: "beam", LocationCode: "A", ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 3, DamageExtentPercent: 20, MoisturePercent: 16, EvidenceDigest: "survey-proof"}
	c, _, err := domain.NewCase(domain.CreateCaseInput{CaseID: "archived", SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: []domain.TimberComponent{component}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.FreezeBaseline("survey", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AssessRisk("engineer", now); err != nil {
		t.Fatal(err)
	}
	zoneInput := domain.ZonePlanInput{ZoneID: "z1", ComponentIDs: []string{"beam"}, Method: "注射", ApprovedParameters: map[string]float64{"dose": 100}, ProtectionConstraints: []string{"隔离"}, ResponsibleID: "field", AcceptanceThresholds: domain.AcceptanceThresholds{MaxActivityCount: 0, MaxAcousticScore: 0.2, MinObservations: 1}}
	if _, err := c.SubmitPlan("planner", []domain.ZonePlanInput{zoneInput}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ApprovePlan("approver", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RecordExecution("z1", "field", domain.ExecutionInput{ActualParameters: map[string]float64{"dose": 100}, EvidenceDigest: "execution-proof"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddObservation("z1", "monitor", domain.ObservationInput{ObservationID: "o1", Method: "trap", VisualFinding: "none", EvidenceDigest: "monitor-proof"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.EvaluateZone("z1", "engineer"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ReviewCase("reviewer", "pass", "证据完整", true, now); err != nil {
		t.Fatal(err)
	}

	events := make([]domain.AuditEvent, 9)
	previous := ""
	for index := range events {
		payload := json.RawMessage(`{"step":` + string(rune('1'+index)) + `}`)
		events[index] = domain.AuditEvent{CaseID: c.CaseID, Sequence: int64(index + 1), EventType: "fixture", ActorID: "actor", OccurredAt: now.Add(time.Duration(index) * time.Second), Payload: payload, PreviousDigest: previous, RequestID: "request-" + string(rune('1'+index))}
		events[index].EventDigest = auditDigest(events[index])
		events[index].EventID = events[index].EventDigest[:24]
		previous = events[index].EventDigest
	}
	c.Revision = int64(len(events))
	manifest, digest, err := Build(c, events)
	if err != nil {
		t.Fatal(err)
	}
	c.ArchiveDigest = digest
	zone := c.Plan.Zones[0]
	zoneJSON, _ := json.Marshal(zone)
	attemptJSON, _ := json.Marshal(zone.Attempts[0])
	observationJSON, _ := json.Marshal(zone.Observations[0])
	fixture := reportFixture{c: c, events: events, archive: &store.ArchiveRecord{CaseID: c.CaseID, TerminalRevision: c.Revision, Manifest: manifest, Digest: digest, CreatedAt: now}, facts: &store.NormalizedFacts{
		Components:   []store.NormalizedComponent{{ComponentID: component.ComponentID, HeritageGrade: component.HeritageGrade, ActivityScore: component.ActivityScore, DamageExtent: component.DamageExtentPercent, Moisture: component.MoisturePercent, EvidenceDigest: component.EvidenceDigest}},
		Zones:        []store.NormalizedZone{{ZoneID: zone.ZoneID, Snapshot: zoneJSON}},
		Executions:   []store.NormalizedExecution{{ZoneID: zone.ZoneID, AttemptNumber: 1, EvidenceDigest: zone.Attempts[0].EvidenceDigest, Snapshot: attemptJSON}},
		Observations: []store.NormalizedObservation{{ZoneID: zone.ZoneID, ObservationID: zone.Observations[0].ObservationID, AttemptNumber: 1, Snapshot: observationJSON}},
	}}
	report, err := BuildVerificationReport(context.Background(), fixture, c.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid || len(report.Checks) < 10 {
		t.Fatalf("完整归档报告未通过: %#v", report)
	}

	fixture.events[1].PreviousDigest = "tampered"
	fixture.facts.Observations = nil
	report, err = BuildVerificationReport(context.Background(), fixture, c.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Valid || !hasFailedCheck(report, "event_previous_digest") || !hasFailedCheck(report, "normalized_monitoring_count") {
		t.Fatalf("未同时报告事件链和监测异常: %#v", report.Checks)
	}
}

func hasFailedCheck(report *VerificationReport, code string) bool {
	for _, check := range report.Checks {
		if check.CheckCode == code && check.Status == "fail" {
			return true
		}
	}
	return false
}
