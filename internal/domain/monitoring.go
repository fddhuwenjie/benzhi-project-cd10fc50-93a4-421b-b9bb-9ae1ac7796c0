package domain

import (
	"strings"
	"time"
)

type ObservationInput struct {
	ObservationID  string    `json:"observation_id"`
	ObservedAt     time.Time `json:"observed_at"`
	Method         string    `json:"method"`
	ActivityCount  int       `json:"activity_count"`
	AcousticScore  float64   `json:"acoustic_score"`
	VisualFinding  string    `json:"visual_finding"`
	EvidenceDigest string    `json:"evidence_digest"`
}

func (c *RemediationCase) AddObservation(zoneID, actor string, in ObservationInput, now time.Time) (EventDraft, error) {
	if err := c.assertMutable(); err != nil {
		return EventDraft{}, err
	}
	if c.Status != StatusInExecution && c.Status != StatusMonitoring {
		return EventDraft{}, Gate("monitoring_state", "当前状态不允许登记监测")
	}
	zone, ok := c.Zone(zoneID)
	if !ok {
		return EventDraft{}, NotFound("分区", zoneID)
	}
	last := lastAttempt(zone)
	if last == nil || zone.ExecutionStatus != "executed" {
		return EventDraft{}, Gate("execution_required", "必须先完成本次分区施作")
	}
	if strings.TrimSpace(in.ObservationID) == "" || strings.TrimSpace(in.Method) == "" || strings.TrimSpace(in.EvidenceDigest) == "" || actor == "" {
		return EventDraft{}, Invalid("observation_fields_required", "observation_id、method、evidence_digest 和 actor_id 均为必填")
	}
	if in.Method != "trap" && in.Method != "acoustic" && in.Method != "visual" {
		return EventDraft{}, Invalid("monitoring_method_invalid", "method 必须为 trap、acoustic 或 visual")
	}
	if in.ActivityCount < 0 || in.AcousticScore < 0 {
		return EventDraft{}, Invalid("monitoring_value_invalid", "监测数值不得为负数")
	}
	observedAt := in.ObservedAt.UTC()
	if in.ObservedAt.IsZero() {
		observedAt = now.UTC()
	}
	minimum := last.ExecutedAt.Add(time.Duration(zone.Thresholds.ObservationWindowHours) * time.Hour)
	if observedAt.Before(minimum) {
		return EventDraft{}, Gate("observation_window_open", "观察窗口尚未届满")
	}
	if observedAt.After(now.UTC().Add(5 * time.Minute)) {
		return EventDraft{}, Invalid("observation_time_future", "observed_at 不得显著晚于当前时间")
	}
	for _, z := range c.Plan.Zones {
		for _, observation := range z.Observations {
			if observation.ObservationID == in.ObservationID {
				return EventDraft{}, Conflict("observation_id_exists", "observation_id 已存在")
			}
		}
	}
	zone.Observations = append(zone.Observations, MonitoringObservation{ObservationID: in.ObservationID, ObservedAt: observedAt, Method: in.Method, ActivityCount: in.ActivityCount, AcousticScore: in.AcousticScore, VisualFinding: strings.TrimSpace(in.VisualFinding), EvidenceDigest: in.EvidenceDigest, RecordedBy: actor, AttemptNumber: zone.AttemptNumber})
	zone.MonitoringStatus = "collecting"
	c.Status = StatusMonitoring
	return EventDraft{Type: "monitoring_observation_recorded", ActorID: actor, Payload: map[string]any{"zone_id": zoneID, "observation_id": in.ObservationID, "attempt_number": zone.AttemptNumber, "observed_at": observedAt}}, nil
}

func (c *RemediationCase) EvaluateZone(zoneID, actor string) (EventDraft, error) {
	if err := c.assertMutable(); err != nil {
		return EventDraft{}, err
	}
	if c.Status != StatusMonitoring {
		return EventDraft{}, Gate("evaluation_state", "当前状态不允许判定分区效果")
	}
	zone, ok := c.Zone(zoneID)
	if !ok {
		return EventDraft{}, NotFound("分区", zoneID)
	}
	current := currentObservations(zone)
	if len(current) < zone.Thresholds.MinObservations {
		return EventDraft{}, Gate("observations_insufficient", "分区 %s 当前尝试至少需要 %d 条监测", zoneID, zone.Thresholds.MinObservations)
	}
	passed := true
	reasons := make([]string, 0)
	for _, observation := range current {
		if observation.ActivityCount > zone.Thresholds.MaxActivityCount {
			passed = false
			reasons = append(reasons, "activity_count")
		}
		if observation.AcousticScore > zone.Thresholds.MaxAcousticScore {
			passed = false
			reasons = append(reasons, "acoustic_score")
		}
		if !zone.Thresholds.AllowVisualActivity && strings.TrimSpace(observation.VisualFinding) != "" && observation.VisualFinding != "none" {
			passed = false
			reasons = append(reasons, "visual_finding")
		}
	}
	if passed {
		zone.MonitoringStatus = "passed"
	} else {
		zone.MonitoringStatus = "failed"
		zone.ExecutionStatus = "failed"
		c.Status = StatusRemediationRequired
	}
	if passed {
		allPassed := true
		for i := range c.Plan.Zones {
			if c.Plan.Zones[i].MonitoringStatus != "passed" {
				allPassed = false
				break
			}
		}
		if allPassed {
			c.Status = StatusReadyForReview
		} else {
			c.Status = StatusMonitoring
		}
	}
	return EventDraft{Type: "zone_effect_evaluated", ActorID: actor, Payload: map[string]any{"zone_id": zoneID, "attempt_number": zone.AttemptNumber, "passed": passed, "reasons": reasons}}, nil
}

func currentObservations(zone *TreatmentZone) []MonitoringObservation {
	out := make([]MonitoringObservation, 0)
	for _, item := range zone.Observations {
		if item.AttemptNumber == zone.AttemptNumber {
			out = append(out, item)
		}
	}
	return out
}
