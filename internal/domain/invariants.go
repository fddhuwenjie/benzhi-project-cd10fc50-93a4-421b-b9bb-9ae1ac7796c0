package domain

import "fmt"

func ValidateAggregate(c *RemediationCase) error {
	if c == nil {
		return NewError(KindCorrupt, "aggregate_nil", "案件聚合为空")
	}
	if c.CaseID == "" || c.SiteName == "" || c.BuildingZone == "" || c.SurveyLeadID == "" {
		return corrupt(c, "identity", "案件身份或勘察负责人缺失")
	}
	if c.Revision < 1 {
		return corrupt(c, "revision", "revision 必须大于 0")
	}
	if len(c.Components) == 0 {
		return corrupt(c, "components", "案件没有基线构件")
	}
	componentIDs := make(map[string]bool, len(c.Components))
	for _, component := range c.Components {
		if err := validateComponent(component); err != nil {
			return corrupt(c, "component", err.Error())
		}
		if componentIDs[component.ComponentID] {
			return corrupt(c, "component_duplicate", "构件标识重复")
		}
		componentIDs[component.ComponentID] = true
	}
	if c.Status != StatusDraft && c.BaselineFrozenAt == nil {
		return corrupt(c, "baseline", "非 draft 案件缺少冻结基线时间")
	}
	if statusAtLeastRisk(c.Status) && c.Risk == nil {
		return corrupt(c, "risk", "风险阶段之后缺少风险结果")
	}
	if c.Risk != nil {
		if c.Risk.RuleVersion == "" || c.Risk.AssessedBy == "" || c.Risk.AssessedAt.IsZero() {
			return corrupt(c, "risk_fields", "风险结果字段不完整")
		}
		if c.Risk.Level != RiskLow && c.Risk.Level != RiskModerate && c.Risk.Level != RiskHigh && c.Risk.Level != RiskCritical {
			return corrupt(c, "risk_level", "风险等级无效")
		}
	}
	if statusNeedsPlan(c.Status) && c.Plan == nil {
		return corrupt(c, "plan", "方案阶段之后缺少当前方案")
	}
	if c.Plan != nil {
		if err := validateStoredPlan(c, c.Plan, componentIDs); err != nil {
			return err
		}
		if c.Plan.Version < 1 {
			return corrupt(c, "plan_version", "当前方案版本必须大于 0")
		}
		if len(c.PlanHistory) != c.Plan.Version-1 {
			return corrupt(c, "plan_history", "方案历史数与当前版本不一致")
		}
		for index := range c.PlanHistory {
			if c.PlanHistory[index].Version != index+1 {
				return corrupt(c, "plan_history_version", "方案历史版本不连续")
			}
		}
	}
	if c.Status == StatusPlanReturned {
		if len(c.PlanReturns) == 0 || c.Plan == nil || c.PlanReturns[len(c.PlanReturns)-1].PlanVersion != c.Plan.Version {
			return corrupt(c, "plan_return", "方案退回状态缺少当前版本退回事实")
		}
	}
	for _, returned := range c.PlanReturns {
		if returned.PlanVersion < 1 || returned.ReturnedBy == "" || returned.ReturnedAt.IsZero() || returned.Reason == "" || len(returned.ZoneIDs) == 0 {
			return corrupt(c, "plan_return_fields", "方案退回事实字段不完整")
		}
	}
	if c.Status == StatusPlanApproved && (c.Plan.ApprovedAt == nil || c.Plan.ApprovedBy == "") {
		return corrupt(c, "approval", "已批准状态缺少批准事实")
	}
	if c.Status == StatusReadyForReview || c.Status == StatusArchived || c.Status == StatusReviewReturned {
		for _, zone := range c.Plan.Zones {
			if zone.MonitoringStatus != "passed" {
				return corrupt(c, "zone_closure", fmt.Sprintf("分区 %s 未通过监测", zone.ZoneID))
			}
		}
	}
	if c.Status == StatusReviewReturned {
		if c.Review == nil || c.Review.Decision != "return" {
			return corrupt(c, "review_return", "复核退回状态缺少退回结论")
		}
	}
	if c.Status == StatusArchived {
		if c.FrozenAt == nil || c.Review == nil || c.Review.Decision != "pass" || !c.Review.EvidenceComplete {
			return corrupt(c, "archive_terminal", "归档案件缺少有效通过结论或冻结时间")
		}
	}
	if c.FrozenAt != nil && c.Status != StatusArchived {
		return corrupt(c, "frozen_status", "非归档案件不应存在 frozen_at")
	}
	if c.Review != nil {
		if c.Review.ReviewerID == c.PlanApproverID || c.Review.ReviewerID == c.SurveyLeadID || (c.Plan != nil && c.Review.ReviewerID == c.Plan.SubmittedBy) {
			return corrupt(c, "review_separation", "复核职责分离事实不成立")
		}
	}
	return nil
}

func validateStoredPlan(c *RemediationCase, plan *TreatmentPlan, components map[string]bool) error {
	if plan.SubmittedBy == "" || plan.SubmittedAt.IsZero() || len(plan.Zones) == 0 {
		return corrupt(c, "plan_fields", "当前方案字段不完整")
	}
	zoneIDs := make(map[string]bool, len(plan.Zones))
	assigned := make(map[string]bool, len(components))
	observationIDs := make(map[string]bool)
	for _, zone := range plan.Zones {
		if zone.ZoneID == "" || zone.Method == "" || zone.ResponsibleID == "" || len(zone.ComponentIDs) == 0 {
			return corrupt(c, "zone_fields", "分区字段不完整")
		}
		if zoneIDs[zone.ZoneID] {
			return corrupt(c, "zone_duplicate", "分区标识重复")
		}
		zoneIDs[zone.ZoneID] = true
		for _, componentID := range zone.ComponentIDs {
			if !components[componentID] {
				return corrupt(c, "zone_component_unknown", fmt.Sprintf("分区 %s 引用未知构件", zone.ZoneID))
			}
			if assigned[componentID] {
				return corrupt(c, "zone_component_duplicate", fmt.Sprintf("构件 %s 被重复分区", componentID))
			}
			assigned[componentID] = true
		}
		if len(zone.ApprovedParameters) == 0 || len(zone.ProtectionConstraints) == 0 || zone.Thresholds.MinObservations < 1 {
			return corrupt(c, "zone_approval", fmt.Sprintf("分区 %s 的批准参数或阈值不完整", zone.ZoneID))
		}
		if zone.AttemptNumber != len(zone.Attempts) {
			return corrupt(c, "attempt_count", fmt.Sprintf("分区 %s 的尝试编号与数量不一致", zone.ZoneID))
		}
		for index, attempt := range zone.Attempts {
			if attempt.AttemptNumber != index+1 || attempt.ExecutedBy == "" || attempt.ExecutedAt.IsZero() || attempt.EvidenceDigest == "" || len(attempt.ActualParameters) == 0 {
				return corrupt(c, "attempt_fields", fmt.Sprintf("分区 %s 的施作尝试不完整", zone.ZoneID))
			}
			if attempt.DeviationSeverity != "none" && attempt.DeviationSeverity != "minor" && attempt.DeviationSeverity != "major" {
				return corrupt(c, "attempt_deviation", fmt.Sprintf("分区 %s 的偏差等级无效", zone.ZoneID))
			}
			if c.Review != nil && attempt.ExecutedBy == c.Review.ReviewerID {
				return corrupt(c, "review_execution_separation", "复核员参与了现场施作")
			}
		}
		currentCount := 0
		for _, observation := range zone.Observations {
			if observation.ObservationID == "" || observation.RecordedBy == "" || observation.EvidenceDigest == "" || observation.ObservedAt.IsZero() {
				return corrupt(c, "observation_fields", fmt.Sprintf("分区 %s 的监测事实不完整", zone.ZoneID))
			}
			if observationIDs[observation.ObservationID] {
				return corrupt(c, "observation_duplicate", "监测标识跨分区重复")
			}
			observationIDs[observation.ObservationID] = true
			if observation.AttemptNumber < 1 || observation.AttemptNumber > zone.AttemptNumber {
				return corrupt(c, "observation_attempt", "监测引用了无效施作尝试")
			}
			if observation.AttemptNumber == zone.AttemptNumber {
				currentCount++
			}
		}
		if zone.MonitoringStatus == "passed" && currentCount < zone.Thresholds.MinObservations {
			return corrupt(c, "monitoring_pass", fmt.Sprintf("分区 %s 在监测不足时标记通过", zone.ZoneID))
		}
	}
	if len(assigned) != len(components) {
		return corrupt(c, "plan_coverage", "当前方案未覆盖全部基线构件")
	}
	return nil
}

func statusAtLeastRisk(status CaseStatus) bool {
	return status != StatusDraft && status != StatusBaselineFrozen
}
func statusNeedsPlan(status CaseStatus) bool {
	return status != StatusDraft && status != StatusBaselineFrozen && status != StatusRiskAssessed
}
func corrupt(c *RemediationCase, suffix, message string) error {
	return NewError(KindCorrupt, "aggregate_"+suffix, "案件 %s 聚合损坏: %s", c.CaseID, message)
}
