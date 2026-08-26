package domain

import (
	"fmt"
	"time"
)

const RiskRuleVersion = "heritage-pest-risk-v1"

func (c *RemediationCase) AssessRisk(actor string, now time.Time) (EventDraft, error) {
	if err := c.assertMutable(); err != nil {
		return EventDraft{}, err
	}
	if c.Status != StatusBaselineFrozen {
		return EventDraft{}, Gate("risk_state", "必须先冻结勘察基线")
	}
	if actor == "" {
		return EventDraft{}, Invalid("actor_required", "actor_id 为必填")
	}
	total := 0
	factors := make([]RiskFactor, 0, len(c.Components)*4)
	for _, component := range c.Components {
		gradePoints := map[string]int{"I": 30, "II": 20, "III": 10}[component.HeritageGrade]
		activityPoints := component.ActivityScore * 6
		damagePoints := rangePoints(component.DamageExtentPercent, 10, 20, 30)
		moisturePoints := 0
		if component.MoisturePercent >= 20 {
			moisturePoints = 15
		} else if component.MoisturePercent >= 15 {
			moisturePoints = 8
		}
		componentPoints := gradePoints + activityPoints + damagePoints + moisturePoints
		total += componentPoints
		factors = append(factors,
			RiskFactor{component.ComponentID, "heritage_grade", gradePoints, component.HeritageGrade},
			RiskFactor{component.ComponentID, "activity_score", activityPoints, fmt.Sprintf("%d/5", component.ActivityScore)},
			RiskFactor{component.ComponentID, "damage_extent", damagePoints, fmt.Sprintf("%.2f%%", component.DamageExtentPercent)},
			RiskFactor{component.ComponentID, "moisture", moisturePoints, fmt.Sprintf("%.2f%%", component.MoisturePercent)},
		)
	}
	average := total / len(c.Components)
	level := RiskLow
	switch {
	case average >= 85:
		level = RiskCritical
	case average >= 60:
		level = RiskHigh
	case average >= 35:
		level = RiskModerate
	}
	c.Risk = &RiskAssessment{RuleVersion: RiskRuleVersion, Score: average, Level: level, Factors: factors, AssessedAt: now.UTC(), AssessedBy: actor}
	c.Status = StatusRiskAssessed
	return EventDraft{Type: "risk_assessed", ActorID: actor, Payload: map[string]any{"rule_version": RiskRuleVersion, "score": average, "level": level, "factors": factors}}, nil
}

func rangePoints(value float64, low, medium, high int) int {
	if value >= 50 {
		return high
	}
	if value >= 20 {
		return medium
	}
	if value > 0 {
		return low
	}
	return 0
}
