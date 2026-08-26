package domain

import (
	"testing"
	"time"
)

func TestReviseBaselineComponentsIsAtomicAndFreezes(t *testing.T) {
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	first := TimberComponent{ComponentID: "a", LocationCode: "A", ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 2, DamageExtentPercent: 10, MoisturePercent: 15, EvidenceDigest: "a-proof"}
	second := TimberComponent{ComponentID: "b", LocationCode: "B", ComponentType: "柱", HeritageGrade: "II", PestClue: "虫粪", ActivityScore: 3, DamageExtentPercent: 20, MoisturePercent: 16, EvidenceDigest: "b-proof"}
	c, _, err := NewCase(CreateCaseInput{CaseID: "revision", SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: []TimberComponent{first, second}}, now)
	if err != nil {
		t.Fatal(err)
	}
	replacement := first
	replacement.MoisturePercent = 18
	third := TimberComponent{ComponentID: "c", LocationCode: "C", ComponentType: "檩", HeritageGrade: "III", PestClue: "活虫", ActivityScore: 4, DamageExtentPercent: 30, MoisturePercent: 17, EvidenceDigest: "c-proof"}
	event, err := c.ReviseBaselineComponents("engineer", []ComponentRevisionOperation{{Operation: "replace", Component: &replacement}, {Operation: "remove", ComponentID: "b"}, {Operation: "add", Component: &third}})
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "batch_baseline_components_revised" || len(c.Components) != 2 || c.Components[0].MoisturePercent != 18 || c.Components[1].ComponentID != "c" {
		t.Fatalf("批量校订结果不正确: %#v", c.Components)
	}
	before := cloneComponents(c.Components)
	bad := third
	bad.MoisturePercent = 101
	if _, err := c.ReviseBaselineComponents("engineer", []ComponentRevisionOperation{{Operation: "remove", ComponentID: "a"}, {Operation: "replace", Component: &bad}}); businessCode(err) != "moisture_invalid" {
		t.Fatalf("预期 moisture_invalid，得到 %v", err)
	}
	if c.Components[0] != before[0] || c.Components[1] != before[1] {
		t.Fatal("失败批次改变了构件清单")
	}
	if _, err := c.FreezeBaseline("survey", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ReviseBaselineComponents("engineer", []ComponentRevisionOperation{{Operation: "remove", ComponentID: "c"}}); businessCode(err) != "baseline_components_frozen" {
		t.Fatalf("冻结门禁错误不稳定: %v", err)
	}
}

func TestPlanReturnAndVersionedResubmission(t *testing.T) {
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	c, _, err := NewCase(CreateCaseInput{CaseID: "return", SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: []TimberComponent{{ComponentID: "beam", LocationCode: "A", ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 2, DamageExtentPercent: 10, MoisturePercent: 15, EvidenceDigest: "proof"}}}, now)
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
	if _, err := c.ReturnPlan("planner", "自行退回", []string{"z1"}, now); businessCode(err) != "plan_return_separation" {
		t.Fatalf("提交人自行退回应失败: %v", err)
	}
	event, err := c.ReturnPlan("approver", "补充彩画隔离措施", []string{"z1"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "plan_returned" || c.Status != StatusPlanReturned || len(c.PlanReturns) != 1 {
		t.Fatal("方案退回事实未保留")
	}
	if _, err := c.SubmitPlan("planner", []ZonePlanInput{validZone()}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if c.Plan.Version != 2 || len(c.PlanHistory) != 1 || c.PlanHistory[0].Version != 1 {
		t.Fatal("方案版本未连续递增")
	}
}

func TestExecutionBatchPrevalidationAndSummary(t *testing.T) {
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	components := []TimberComponent{
		{ComponentID: "a", LocationCode: "A", ComponentType: "梁", HeritageGrade: "I", PestClue: "蛀孔", ActivityScore: 2, DamageExtentPercent: 10, MoisturePercent: 15, EvidenceDigest: "a"},
		{ComponentID: "b", LocationCode: "B", ComponentType: "柱", HeritageGrade: "II", PestClue: "虫粪", ActivityScore: 3, DamageExtentPercent: 20, MoisturePercent: 16, EvidenceDigest: "b"},
	}
	c, _, err := NewCase(CreateCaseInput{CaseID: "batch", SiteName: "古建", BuildingZone: "正殿", SurveyLeadID: "survey", Components: components}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.FreezeBaseline("survey", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AssessRisk("engineer", now); err != nil {
		t.Fatal(err)
	}
	zones := []ZonePlanInput{
		{ZoneID: "z1", ComponentIDs: []string{"a"}, Method: "注射", ApprovedParameters: map[string]float64{"dose": 100}, ProtectionConstraints: []string{"隔离"}, ResponsibleID: "field", AcceptanceThresholds: AcceptanceThresholds{MinObservations: 1}},
		{ZoneID: "z2", ComponentIDs: []string{"b"}, Method: "注射", ApprovedParameters: map[string]float64{"dose": 100}, ProtectionConstraints: []string{"隔离"}, ResponsibleID: "field", AcceptanceThresholds: AcceptanceThresholds{MinObservations: 1}},
	}
	if _, err := c.SubmitPlan("planner", zones, now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ApprovePlan("approver", now); err != nil {
		t.Fatal(err)
	}
	invalid := []ZoneExecutionInput{
		{ZoneID: "z1", Execution: ExecutionInput{ActualParameters: map[string]float64{"dose": 40}, EvidenceDigest: "major"}},
		{ZoneID: "z2", Execution: ExecutionInput{ActualParameters: map[string]float64{"dose": 100}}},
	}
	if _, err := c.RecordExecutionBatch("field", invalid, now); businessCode(err) != "execution_evidence_required" {
		t.Fatalf("预期证据错误: %v", err)
	}
	if c.Plan.Zones[0].AttemptNumber != 0 || c.Plan.Zones[1].AttemptNumber != 0 {
		t.Fatal("失败批次产生了尝试")
	}
	invalid[1].Execution.EvidenceDigest = "ok"
	event, err := c.RecordExecutionBatch("field", invalid, now)
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "zone_execution_batch_recorded" || c.Status != StatusSuspended || c.Plan.Zones[0].AttemptNumber != 1 || c.Plan.Zones[1].AttemptNumber != 1 {
		t.Fatal("成功批次未原子落入两个分区")
	}
	summary := BuildMonitoringSummary(c, now)
	if summary.PendingRemediation != 1 || summary.Collecting != 1 || summary.ReviewReady {
		t.Fatalf("监测摘要分类错误: %#v", summary)
	}
}
