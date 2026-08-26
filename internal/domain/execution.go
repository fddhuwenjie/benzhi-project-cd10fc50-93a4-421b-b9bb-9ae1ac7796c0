package domain

import (
	"math"
	"sort"
	"strings"
	"time"
)

type ExecutionInput struct {
	ExecutedAt       time.Time          `json:"executed_at"`
	ActualParameters map[string]float64 `json:"actual_parameters"`
	EvidenceDigest   string             `json:"evidence_digest"`
	DeviationNote    string             `json:"deviation_note"`
}

const MaxExecutionBatchSize = 50

type ZoneExecutionInput struct {
	ZoneID    string         `json:"zone_id"`
	Execution ExecutionInput `json:"execution"`
}

type preparedExecution struct {
	zone       *TreatmentZone
	executedAt time.Time
	severity   string
	deviations map[string]float64
	input      ExecutionInput
}

func (c *RemediationCase) RecordExecution(zoneID, actor string, input ExecutionInput, now time.Time) (EventDraft, error) {
	prepared, err := c.prepareExecution(zoneID, actor, input, now)
	if err != nil {
		return EventDraft{}, err
	}
	c.applyExecution(actor, prepared)
	if prepared.severity == "major" {
		c.Status = StatusSuspended
	} else {
		c.Status = StatusInExecution
	}
	return EventDraft{Type: "zone_execution_recorded", ActorID: actor, Payload: map[string]any{"zone_id": zoneID, "attempt_number": prepared.zone.AttemptNumber, "deviation_severity": prepared.severity, "deviations": prepared.deviations, "executed_at": prepared.executedAt}}, nil
}

func (c *RemediationCase) RecordExecutionBatch(actor string, inputs []ZoneExecutionInput, now time.Time) (EventDraft, error) {
	if len(inputs) < 2 || len(inputs) > MaxExecutionBatchSize {
		return EventDraft{}, Invalid("execution_batch_count", "施作批次必须包含 2 到 %d 个分区", MaxExecutionBatchSize)
	}
	seen := make(map[string]bool, len(inputs))
	prepared := make([]preparedExecution, 0, len(inputs))
	for index, input := range inputs {
		zoneID := strings.TrimSpace(input.ZoneID)
		if zoneID == "" {
			return EventDraft{}, Invalid("zone_id_required", "第 %d 项 zone_id 为必填", index)
		}
		if seen[zoneID] {
			return EventDraft{}, Invalid("execution_batch_duplicate_zone", "施作批次中的分区 %s 重复", zoneID)
		}
		seen[zoneID] = true
		item, err := c.prepareExecution(zoneID, actor, input.Execution, now)
		if err != nil {
			return EventDraft{}, err
		}
		prepared = append(prepared, item)
	}
	results := make([]map[string]any, 0, len(prepared))
	hasMajor := false
	for _, item := range prepared {
		c.applyExecution(actor, item)
		hasMajor = hasMajor || item.severity == "major"
		results = append(results, map[string]any{"zone_id": item.zone.ZoneID, "attempt_number": item.zone.AttemptNumber, "deviation_severity": item.severity, "deviations": item.deviations, "executed_at": item.executedAt})
	}
	sort.Slice(results, func(i, j int) bool { return results[i]["zone_id"].(string) < results[j]["zone_id"].(string) })
	if hasMajor {
		c.Status = StatusSuspended
	} else {
		c.Status = StatusInExecution
	}
	return EventDraft{Type: "zone_execution_batch_recorded", ActorID: actor, Payload: map[string]any{"results": results}}, nil
}

func (c *RemediationCase) prepareExecution(zoneID, actor string, input ExecutionInput, now time.Time) (preparedExecution, error) {
	if err := c.assertMutable(); err != nil {
		return preparedExecution{}, err
	}
	if c.Status != StatusPlanApproved && c.Status != StatusInExecution && c.Status != StatusRemediationRequired && c.Status != StatusMonitoring {
		return preparedExecution{}, Gate("execution_state", "当前状态不允许登记施作")
	}
	zone, ok := c.Zone(zoneID)
	if !ok {
		return preparedExecution{}, NotFound("分区", zoneID)
	}
	if actor == "" || actor != zone.ResponsibleID {
		return preparedExecution{}, Gate("field_responsibility", "仅分区责任人可登记施作")
	}
	if zone.ExecutionStatus == "executed" && zone.MonitoringStatus != "failed" {
		return preparedExecution{}, Conflict("zone_already_executed", "分区已完成本次施作")
	}
	if zone.MonitoringStatus == "failed" {
		last := lastAttempt(zone)
		if last == nil || strings.TrimSpace(last.RemediationDecision) == "" {
			return preparedExecution{}, Gate("remediation_decision_required", "监测失败后必须先形成整改决定")
		}
	}
	if strings.TrimSpace(input.EvidenceDigest) == "" || len(input.ActualParameters) == 0 {
		return preparedExecution{}, Invalid("execution_evidence_required", "实际参数和证据摘要均为必填")
	}
	for name, value := range input.ActualParameters {
		if strings.TrimSpace(name) == "" || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return preparedExecution{}, Invalid("execution_parameter_invalid", "分区 %s 包含无效实际参数", zoneID)
		}
	}
	executedAt := input.ExecutedAt.UTC()
	if input.ExecutedAt.IsZero() {
		executedAt = now.UTC()
	}
	if executedAt.After(now.UTC().Add(5 * time.Minute)) {
		return preparedExecution{}, Invalid("execution_time_future", "executed_at 不得显著晚于当前时间")
	}
	severity, deviations := compareParameters(zone.ApprovedParameters, input.ActualParameters)
	return preparedExecution{zone: zone, executedAt: executedAt, severity: severity, deviations: deviations, input: input}, nil
}

func (c *RemediationCase) applyExecution(actor string, prepared preparedExecution) {
	zone := prepared.zone
	zone.AttemptNumber++
	zone.Attempts = append(zone.Attempts, ExecutionAttempt{AttemptNumber: zone.AttemptNumber, ExecutedAt: prepared.executedAt, ExecutedBy: actor, ActualParameters: cloneFloatMap(prepared.input.ActualParameters), EvidenceDigest: strings.TrimSpace(prepared.input.EvidenceDigest), DeviationSeverity: prepared.severity, DeviationNote: strings.TrimSpace(prepared.input.DeviationNote)})
	zone.ExecutionStatus = "executed"
	zone.DeviationSeverity = prepared.severity
	zone.MonitoringStatus = "pending"
	c.FieldLeadID = actor
}

func compareParameters(approved, actual map[string]float64) (string, map[string]float64) {
	severity := "none"
	deviations := make(map[string]float64)
	for name, expected := range approved {
		value, ok := actual[name]
		if !ok {
			deviations[name] = -1
			return "major", deviations
		}
		delta := math.Abs(value-expected) / math.Max(math.Abs(expected), 1)
		if delta > 0.20 {
			severity = "major"
		} else if delta > 0.05 && severity == "none" {
			severity = "minor"
		}
		if delta > 0.05 {
			deviations[name] = delta
		}
	}
	for name := range actual {
		if _, ok := approved[name]; !ok {
			deviations[name] = 1
			severity = "major"
		}
	}
	return severity, deviations
}

func (c *RemediationCase) DecideRemediation(zoneID, actor, decision string) (EventDraft, error) {
	if err := c.assertMutable(); err != nil {
		return EventDraft{}, err
	}
	if c.Status != StatusSuspended && c.Status != StatusRemediationRequired {
		return EventDraft{}, Gate("remediation_state", "当前案件不需要整改决定")
	}
	zone, ok := c.Zone(zoneID)
	if !ok {
		return EventDraft{}, NotFound("分区", zoneID)
	}
	last := lastAttempt(zone)
	if last == nil {
		return EventDraft{}, Gate("execution_required", "分区尚无施作尝试")
	}
	if strings.TrimSpace(decision) == "" {
		return EventDraft{}, Invalid("remediation_decision_required", "整改决定不能为空")
	}
	if last.DeviationSeverity != "major" && zone.MonitoringStatus != "failed" {
		return EventDraft{}, Gate("zone_remediation_not_required", "分区当前尝试不需要整改决定")
	}
	if last.RemediationDecision != "" {
		return EventDraft{}, Conflict("remediation_decided", "本次尝试已有整改决定")
	}
	if actor == last.ExecutedBy {
		return EventDraft{}, Gate("remediation_separation", "施作人员不能批准自己的整改决定")
	}
	last.RemediationDecision = strings.TrimSpace(decision)
	zone.ExecutionStatus = "remediation_approved"
	c.Status = StatusRemediationRequired
	return EventDraft{Type: "remediation_decided", ActorID: actor, Payload: map[string]any{"zone_id": zoneID, "attempt_number": zone.AttemptNumber, "decision": decision}}, nil
}

func lastAttempt(zone *TreatmentZone) *ExecutionAttempt {
	if len(zone.Attempts) == 0 {
		return nil
	}
	return &zone.Attempts[len(zone.Attempts)-1]
}
