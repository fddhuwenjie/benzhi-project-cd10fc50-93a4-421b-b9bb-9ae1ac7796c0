package domain

import (
	"sort"
	"strings"
	"time"
)

type ZonePlanInput struct {
	ZoneID                string               `json:"zone_id"`
	ComponentIDs          []string             `json:"component_ids"`
	Method                string               `json:"method"`
	ApprovedParameters    map[string]float64   `json:"approved_parameters"`
	ProtectionConstraints []string             `json:"protection_constraints"`
	ResponsibleID         string               `json:"responsible_id"`
	AcceptanceThresholds  AcceptanceThresholds `json:"acceptance_thresholds"`
}

func (c *RemediationCase) SubmitPlan(actor string, zones []ZonePlanInput, now time.Time) (EventDraft, error) {
	if err := c.assertMutable(); err != nil {
		return EventDraft{}, err
	}
	if c.Status != StatusRiskAssessed && c.Status != StatusPlanReturned && c.Status != StatusReviewReturned {
		return EventDraft{}, Gate("plan_state", "当前状态不允许提交方案")
	}
	if actor == "" {
		return EventDraft{}, Invalid("actor_required", "actor_id 为必填")
	}
	if len(zones) == 0 {
		return EventDraft{}, Invalid("zones_required", "方案至少包含一个分区")
	}
	known := make(map[string]bool, len(c.Components))
	for _, component := range c.Components {
		known[component.ComponentID] = true
	}
	assigned := make(map[string]string, len(c.Components))
	zoneIDs := make(map[string]bool, len(zones))
	planZones := make([]TreatmentZone, 0, len(zones))
	for _, in := range zones {
		if err := validateZonePlan(in); err != nil {
			return EventDraft{}, err
		}
		zoneID := strings.TrimSpace(in.ZoneID)
		if zoneIDs[zoneID] {
			return EventDraft{}, Invalid("duplicate_zone", "分区 %s 重复", zoneID)
		}
		zoneIDs[zoneID] = true
		componentIDs := append([]string(nil), in.ComponentIDs...)
		sort.Strings(componentIDs)
		for _, id := range componentIDs {
			if !known[id] {
				return EventDraft{}, Invalid("unknown_component", "分区 %s 引用了未知构件 %s", zoneID, id)
			}
			if previous := assigned[id]; previous != "" {
				return EventDraft{}, Invalid("component_multiple_zones", "构件 %s 同时属于 %s 和 %s", id, previous, zoneID)
			}
			assigned[id] = zoneID
		}
		planZones = append(planZones, TreatmentZone{ZoneID: zoneID, ComponentIDs: componentIDs, Method: strings.TrimSpace(in.Method), ApprovedParameters: cloneFloatMap(in.ApprovedParameters), ProtectionConstraints: append([]string(nil), in.ProtectionConstraints...), ResponsibleID: in.ResponsibleID, Thresholds: in.AcceptanceThresholds, ExecutionStatus: "pending", MonitoringStatus: "pending"})
	}
	if len(assigned) != len(known) {
		return EventDraft{}, Gate("plan_coverage_incomplete", "方案必须且只能覆盖全部基线构件")
	}
	sort.Slice(planZones, func(i, j int) bool { return planZones[i].ZoneID < planZones[j].ZoneID })
	version := 1
	if c.Plan != nil {
		version = c.Plan.Version + 1
		c.PlanHistory = append(c.PlanHistory, clonePlan(*c.Plan))
	}
	if c.Review != nil {
		c.ReviewHistory = append(c.ReviewHistory, *c.Review)
		c.Review = nil
		c.ReviewerID = ""
	}
	c.Plan = &TreatmentPlan{Version: version, SubmittedBy: actor, SubmittedAt: now.UTC(), Zones: planZones}
	c.Status = StatusPlanSubmitted
	return EventDraft{Type: "plan_submitted", ActorID: actor, Payload: map[string]any{"plan_version": version, "zone_count": len(planZones), "zones": zoneIDs}}, nil
}

func (c *RemediationCase) ReturnPlan(actor, reason string, zoneIDs []string, now time.Time) (EventDraft, error) {
	if err := c.assertMutable(); err != nil {
		return EventDraft{}, err
	}
	if c.Status != StatusPlanSubmitted || c.Plan == nil {
		return EventDraft{}, Gate("plan_return_state", "仅待批准方案可退回")
	}
	if strings.TrimSpace(actor) == "" {
		return EventDraft{}, Invalid("actor_required", "actor_id 为必填")
	}
	if actor == c.Plan.SubmittedBy {
		return EventDraft{}, Gate("plan_return_separation", "方案提交人不能退回自己的方案")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return EventDraft{}, Invalid("plan_return_reason_required", "退回原因不能为空")
	}
	if len(zoneIDs) == 0 {
		return EventDraft{}, Invalid("plan_return_zones_required", "至少指定一个需修订分区")
	}
	known := make(map[string]bool, len(c.Plan.Zones))
	for _, zone := range c.Plan.Zones {
		known[zone.ZoneID] = true
	}
	unique := make(map[string]bool, len(zoneIDs))
	cleaned := make([]string, 0, len(zoneIDs))
	for _, zoneID := range zoneIDs {
		zoneID = strings.TrimSpace(zoneID)
		if zoneID == "" || !known[zoneID] {
			return EventDraft{}, Invalid("plan_return_zone_unknown", "需修订分区 %s 不属于当前方案", zoneID)
		}
		if unique[zoneID] {
			return EventDraft{}, Invalid("plan_return_zone_duplicate", "需修订分区 %s 重复", zoneID)
		}
		unique[zoneID] = true
		cleaned = append(cleaned, zoneID)
	}
	sort.Strings(cleaned)
	decision := PlanReturnDecision{PlanVersion: c.Plan.Version, ReturnedBy: actor, ReturnedAt: now.UTC(), Reason: reason, ZoneIDs: cleaned}
	c.PlanReturns = append(c.PlanReturns, decision)
	c.Status = StatusPlanReturned
	return EventDraft{Type: "plan_returned", ActorID: actor, Payload: map[string]any{"plan_version": c.Plan.Version, "reason": reason, "zone_ids": cleaned, "returned_at": decision.ReturnedAt}}, nil
}

func validateZonePlan(in ZonePlanInput) error {
	if strings.TrimSpace(in.ZoneID) == "" || strings.TrimSpace(in.Method) == "" || strings.TrimSpace(in.ResponsibleID) == "" {
		return Invalid("zone_fields_required", "zone_id、method 和 responsible_id 均为必填")
	}
	if len(in.ComponentIDs) == 0 {
		return Invalid("zone_components_required", "分区 %s 未包含构件", in.ZoneID)
	}
	if len(in.ApprovedParameters) == 0 {
		return Invalid("approved_parameters_required", "分区 %s 缺少批准参数", in.ZoneID)
	}
	if len(in.ProtectionConstraints) == 0 {
		return Invalid("protection_constraints_required", "分区 %s 缺少保护约束", in.ZoneID)
	}
	t := in.AcceptanceThresholds
	if t.MaxActivityCount < 0 || t.MaxAcousticScore < 0 || t.MinObservations < 1 || t.ObservationWindowHours < 0 {
		return Invalid("threshold_invalid", "分区 %s 的验收阈值无效", in.ZoneID)
	}
	for name, value := range in.ApprovedParameters {
		if strings.TrimSpace(name) == "" || value < 0 {
			return Invalid("approved_parameter_invalid", "分区 %s 包含无效批准参数", in.ZoneID)
		}
	}
	return nil
}

func (c *RemediationCase) ApprovePlan(actor string, now time.Time) (EventDraft, error) {
	if err := c.assertMutable(); err != nil {
		return EventDraft{}, err
	}
	if c.Status != StatusPlanSubmitted || c.Plan == nil {
		return EventDraft{}, Gate("approval_state", "没有待批准方案")
	}
	if actor == "" {
		return EventDraft{}, Invalid("actor_required", "actor_id 为必填")
	}
	if actor == c.Plan.SubmittedBy {
		return EventDraft{}, Gate("approval_separation", "方案提交人与批准人必须分离")
	}
	t := now.UTC()
	c.Plan.ApprovedBy = actor
	c.Plan.ApprovedAt = &t
	c.PlanApproverID = actor
	c.Status = StatusPlanApproved
	return EventDraft{Type: "plan_approved", ActorID: actor, Payload: map[string]any{"approved_at": t, "zone_count": len(c.Plan.Zones)}}, nil
}

func cloneFloatMap(src map[string]float64) map[string]float64 {
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func clonePlan(src TreatmentPlan) TreatmentPlan {
	dst := src
	dst.Zones = make([]TreatmentZone, len(src.Zones))
	for i, zone := range src.Zones {
		dst.Zones[i] = zone
		dst.Zones[i].ComponentIDs = append([]string(nil), zone.ComponentIDs...)
		dst.Zones[i].ApprovedParameters = cloneFloatMap(zone.ApprovedParameters)
		dst.Zones[i].ProtectionConstraints = append([]string(nil), zone.ProtectionConstraints...)
		dst.Zones[i].Attempts = append([]ExecutionAttempt(nil), zone.Attempts...)
		dst.Zones[i].Observations = append([]MonitoringObservation(nil), zone.Observations...)
		for j := range dst.Zones[i].Attempts {
			dst.Zones[i].Attempts[j].ActualParameters = cloneFloatMap(zone.Attempts[j].ActualParameters)
		}
	}
	return dst
}
