package domain

import (
	"errors"
	"testing"
	"time"
)

func TestMajorDeviationRequiresIndependentRemediation(t *testing.T) {
	c, now := preparedCase(t)
	event, err := c.RecordExecution("z1", "field", ExecutionInput{ActualParameters: map[string]float64{"dose": 40}, EvidenceDigest: "exec-major"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusSuspended || event.Type != "zone_execution_recorded" {
		t.Fatalf("重大偏差未暂停案件: %s", c.Status)
	}
	if _, err := c.DecideRemediation("z1", "field", "调整剂量后重做"); businessCode(err) != "remediation_separation" {
		t.Fatalf("施作人自行批准整改应失败: %v", err)
	}
	if _, err := c.DecideRemediation("z1", "quality", "调整剂量后重做"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RecordExecution("z1", "field", ExecutionInput{ActualParameters: map[string]float64{"dose": 100}, EvidenceDigest: "exec-fixed"}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	zone, _ := c.Zone("z1")
	if zone.AttemptNumber != 2 || len(zone.Attempts) != 2 {
		t.Fatalf("整改重试未保留旧尝试: %#v", zone.Attempts)
	}
	if _, err := c.AddObservation("z1", "monitor", ObservationInput{ObservationID: "o1", Method: "trap", EvidenceDigest: "monitor", VisualFinding: "none"}, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.EvaluateZone("z1", "engineer"); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusReadyForReview {
		t.Fatalf("预期 ready_for_review，得到 %s", c.Status)
	}
	if _, err := c.ReviewCase("approver", "pass", "通过", true, now.Add(3*time.Minute)); businessCode(err) != "review_separation" {
		t.Fatalf("批准人担任复核员应失败: %v", err)
	}
	if _, err := c.ReviewCase("reviewer", "pass", "证据完整", true, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusArchived || c.FrozenAt == nil {
		t.Fatalf("复核通过未冻结案件")
	}
	if _, err := c.AssessRisk("engineer", now); businessCode(err) != "case_frozen" {
		t.Fatalf("归档后变更应失败: %v", err)
	}
}

func TestMonitoringFailureKeepsAttemptEvidence(t *testing.T) {
	c, now := preparedCase(t)
	if _, err := c.RecordExecution("z1", "field", ExecutionInput{ActualParameters: map[string]float64{"dose": 100}, EvidenceDigest: "exec"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddObservation("z1", "monitor", ObservationInput{ObservationID: "bad", Method: "trap", ActivityCount: 2, EvidenceDigest: "bad-monitor", VisualFinding: "none"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.EvaluateZone("z1", "engineer"); err != nil {
		t.Fatal(err)
	}
	zone, _ := c.Zone("z1")
	if c.Status != StatusRemediationRequired || zone.MonitoringStatus != "failed" {
		t.Fatalf("未进入监测整改状态")
	}
	if _, err := c.RecordExecution("z1", "field", ExecutionInput{ActualParameters: map[string]float64{"dose": 100}, EvidenceDigest: "too-soon"}, now); businessCode(err) != "remediation_decision_required" {
		t.Fatalf("没有整改决定时不应重做: %v", err)
	}
	if _, err := c.DecideRemediation("z1", "quality", "增加隔离并重做"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RecordExecution("z1", "field", ExecutionInput{ActualParameters: map[string]float64{"dose": 100}, EvidenceDigest: "second"}, now); err != nil {
		t.Fatal(err)
	}
	if len(zone.Attempts) != 2 {
		t.Fatalf("预期保留两个尝试")
	}
}

func TestReviewReturnPreservesPlanVersion(t *testing.T) {
	c, now := preparedCase(t)
	if _, err := c.RecordExecution("z1", "field", ExecutionInput{ActualParameters: map[string]float64{"dose": 100}, EvidenceDigest: "exec"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddObservation("z1", "monitor", ObservationInput{ObservationID: "ok", Method: "trap", EvidenceDigest: "ok", VisualFinding: "none"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.EvaluateZone("z1", "engineer"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ReviewCase("reviewer", "return", "补充分区保护约束", false, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SubmitPlan("planner", []ZonePlanInput{validZone()}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if c.Plan.Version != 2 || len(c.PlanHistory) != 1 || len(c.ReviewHistory) != 1 {
		t.Fatalf("退回历史未保留: plan=%d plans=%d reviews=%d", c.Plan.Version, len(c.PlanHistory), len(c.ReviewHistory))
	}
}

func preparedCase(t *testing.T) (*RemediationCase, time.Time) {
	t.Helper()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	c, _, err := NewCase(CreateCaseInput{CaseID: "c1", SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: []TimberComponent{{ComponentID: "beam", LocationCode: "A1", ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 4, DamageExtentPercent: 20, MoisturePercent: 16, EvidenceDigest: "survey-proof"}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.FreezeBaseline("survey", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AssessRisk("engineer", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SubmitPlan("planner", []ZonePlanInput{validZone()}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ApprovePlan("approver", now); err != nil {
		t.Fatal(err)
	}
	return c, now
}

func validZone() ZonePlanInput {
	return ZonePlanInput{ZoneID: "z1", ComponentIDs: []string{"beam"}, Method: "注射", ApprovedParameters: map[string]float64{"dose": 100}, ProtectionConstraints: []string{"保护彩画"}, ResponsibleID: "field", AcceptanceThresholds: AcceptanceThresholds{MaxActivityCount: 0, MaxAcousticScore: 0.2, MinObservations: 1}}
}

func businessCode(err error) string {
	var target *BusinessError
	if errors.As(err, &target) {
		return target.Code
	}
	return ""
}
