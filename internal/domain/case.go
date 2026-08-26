package domain

import (
	"strings"
	"time"
)

type CreateCaseInput struct {
	CaseID       string
	SiteName     string
	BuildingZone string
	SurveyLeadID string
	Components   []TimberComponent
}

func NewCase(in CreateCaseInput, now time.Time) (*RemediationCase, EventDraft, error) {
	if strings.TrimSpace(in.CaseID) == "" || strings.TrimSpace(in.SiteName) == "" || strings.TrimSpace(in.BuildingZone) == "" {
		return nil, EventDraft{}, Invalid("case_identity_required", "case_id、site_name 和 building_zone 均为必填")
	}
	if strings.TrimSpace(in.SurveyLeadID) == "" {
		return nil, EventDraft{}, Invalid("survey_lead_required", "survey_lead_id 为必填")
	}
	if len(in.Components) == 0 {
		return nil, EventDraft{}, Invalid("components_required", "至少登记一个木构件")
	}
	seen := make(map[string]struct{}, len(in.Components))
	for i := range in.Components {
		if err := validateComponent(in.Components[i]); err != nil {
			return nil, EventDraft{}, err
		}
		if _, ok := seen[in.Components[i].ComponentID]; ok {
			return nil, EventDraft{}, Invalid("duplicate_component", "构件 %s 重复", in.Components[i].ComponentID)
		}
		seen[in.Components[i].ComponentID] = struct{}{}
	}
	c := &RemediationCase{
		CaseID: in.CaseID, SiteName: strings.TrimSpace(in.SiteName), BuildingZone: strings.TrimSpace(in.BuildingZone),
		Status: StatusDraft, Revision: 1, SurveyLeadID: in.SurveyLeadID, CreatedAt: now.UTC(), Components: cloneComponents(in.Components),
	}
	e := EventDraft{Type: "case_created", ActorID: in.SurveyLeadID, Payload: map[string]any{"component_count": len(in.Components), "site_name": c.SiteName}}
	return c, e, nil
}

func validateComponent(c TimberComponent) error {
	if strings.TrimSpace(c.ComponentID) == "" || strings.TrimSpace(c.LocationCode) == "" || strings.TrimSpace(c.ComponentType) == "" {
		return Invalid("component_identity_required", "构件标识、位置编码和类型均为必填")
	}
	if c.HeritageGrade != "I" && c.HeritageGrade != "II" && c.HeritageGrade != "III" {
		return Invalid("heritage_grade_invalid", "构件 %s 的 heritage_grade 必须为 I、II 或 III", c.ComponentID)
	}
	if c.ActivityScore < 0 || c.ActivityScore > 5 {
		return Invalid("activity_score_invalid", "构件 %s 的 activity_score 必须处于 0 到 5", c.ComponentID)
	}
	if c.DamageExtentPercent < 0 || c.DamageExtentPercent > 100 {
		return Invalid("damage_extent_invalid", "构件 %s 的 damage_extent_percent 必须处于 0 到 100", c.ComponentID)
	}
	if c.MoisturePercent < 0 || c.MoisturePercent > 100 {
		return Invalid("moisture_invalid", "构件 %s 的 moisture_percent 必须处于 0 到 100", c.ComponentID)
	}
	if strings.TrimSpace(c.PestClue) == "" || strings.TrimSpace(c.EvidenceDigest) == "" {
		return Invalid("component_evidence_required", "构件 %s 缺少虫种线索或证据摘要", c.ComponentID)
	}
	return nil
}

func (c *RemediationCase) assertMutable() error {
	if c.Status == StatusArchived || c.FrozenAt != nil {
		return Conflict("case_frozen", "案件已归档冻结，禁止修改")
	}
	return nil
}

func (c *RemediationCase) FreezeBaseline(actor string, now time.Time) (EventDraft, error) {
	if err := c.assertMutable(); err != nil {
		return EventDraft{}, err
	}
	if c.Status != StatusDraft {
		return EventDraft{}, Gate("baseline_state", "仅 draft 案件可冻结勘察基线")
	}
	if actor != c.SurveyLeadID {
		return EventDraft{}, Gate("survey_lead_only", "仅登记的勘察负责人可冻结基线")
	}
	for _, component := range c.Components {
		if err := validateComponent(component); err != nil {
			return EventDraft{}, err
		}
	}
	t := now.UTC()
	c.BaselineFrozenAt = &t
	c.Status = StatusBaselineFrozen
	return EventDraft{Type: "baseline_frozen", ActorID: actor, Payload: map[string]any{"component_count": len(c.Components), "frozen_at": t}}, nil
}

func (c *RemediationCase) Component(id string) (*TimberComponent, bool) {
	for i := range c.Components {
		if c.Components[i].ComponentID == id {
			return &c.Components[i], true
		}
	}
	return nil, false
}

func (c *RemediationCase) Zone(id string) (*TreatmentZone, bool) {
	if c.Plan == nil {
		return nil, false
	}
	for i := range c.Plan.Zones {
		if c.Plan.Zones[i].ZoneID == id {
			return &c.Plan.Zones[i], true
		}
	}
	return nil, false
}

func cloneComponents(src []TimberComponent) []TimberComponent {
	dst := make([]TimberComponent, len(src))
	copy(dst, src)
	return dst
}
