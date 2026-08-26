package domain

import (
	"strings"
	"time"
)

func (c *RemediationCase) ReviewCase(actor, decision, findings string, evidenceComplete bool, now time.Time) (EventDraft, error) {
	if err := c.assertMutable(); err != nil {
		return EventDraft{}, err
	}
	if c.Status != StatusReadyForReview {
		return EventDraft{}, Gate("review_state", "仅全部分区通过后可复核")
	}
	if actor == "" {
		return EventDraft{}, Invalid("reviewer_required", "reviewer_id 为必填")
	}
	if actor == c.PlanApproverID || actor == c.Plan.SubmittedBy || actor == c.SurveyLeadID {
		return EventDraft{}, Gate("review_separation", "复核员不得参与勘察、方案提交或批准")
	}
	for _, zone := range c.Plan.Zones {
		if zone.MonitoringStatus != "passed" {
			return EventDraft{}, Gate("zones_not_closed", "分区 %s 尚未闭合", zone.ZoneID)
		}
		for _, attempt := range zone.Attempts {
			if attempt.ExecutedBy == actor {
				return EventDraft{}, Gate("review_separation", "复核员不得参与现场施作")
			}
		}
		if len(zone.Attempts) == 0 || len(zone.Observations) == 0 {
			return EventDraft{}, Gate("zone_evidence_incomplete", "分区 %s 缺少施作或监测证据", zone.ZoneID)
		}
	}
	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != "pass" && decision != "return" {
		return EventDraft{}, Invalid("review_decision_invalid", "decision 必须为 pass 或 return")
	}
	if strings.TrimSpace(findings) == "" {
		return EventDraft{}, Invalid("review_findings_required", "复核意见不能为空")
	}
	if decision == "pass" && !evidenceComplete {
		return EventDraft{}, Gate("review_evidence_incomplete", "证据不完整时不能签发通过结论")
	}
	c.ReviewerID = actor
	c.Review = &ReviewDecision{ReviewerID: actor, Decision: decision, Findings: findings, EvidenceComplete: evidenceComplete, DecidedAt: now.UTC()}
	if decision == "return" {
		c.Status = StatusReviewReturned
	} else {
		c.Status = StatusArchived
		t := now.UTC()
		c.FrozenAt = &t
	}
	return EventDraft{Type: "review_decided", ActorID: actor, Payload: map[string]any{"decision": decision, "findings": findings, "evidence_complete": evidenceComplete}}, nil
}
