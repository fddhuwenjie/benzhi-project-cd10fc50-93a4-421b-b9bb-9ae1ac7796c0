package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"timber-pest-remediation-ledger/internal/domain"
)

const ManifestVersion = "timber-remediation-archive-v1"

type Manifest struct {
	ManifestVersion  string         `json:"manifest_version"`
	CaseID           string         `json:"case_id"`
	TerminalRevision int64          `json:"terminal_revision"`
	Case             archiveCase    `json:"case"`
	Events           []archiveEvent `json:"events"`
}

type archiveCase struct {
	SiteName         string                      `json:"site_name"`
	BuildingZone     string                      `json:"building_zone"`
	Status           domain.CaseStatus           `json:"status"`
	SurveyLeadID     string                      `json:"survey_lead_id"`
	PlanApproverID   string                      `json:"plan_approver_id"`
	FieldLeadID      string                      `json:"field_lead_id"`
	ReviewerID       string                      `json:"reviewer_id"`
	CreatedAt        string                      `json:"created_at"`
	BaselineFrozenAt string                      `json:"baseline_frozen_at"`
	FrozenAt         string                      `json:"frozen_at"`
	Components       []domain.TimberComponent    `json:"components"`
	Risk             *domain.RiskAssessment      `json:"risk"`
	Plan             *domain.TreatmentPlan       `json:"plan"`
	PlanHistory      []domain.TreatmentPlan      `json:"plan_history"`
	PlanReturns      []domain.PlanReturnDecision `json:"plan_returns"`
	Review           *domain.ReviewDecision      `json:"review"`
	ReviewHistory    []domain.ReviewDecision     `json:"review_history"`
}

type archiveEvent struct {
	EventID        string          `json:"event_id"`
	Sequence       int64           `json:"sequence"`
	EventType      string          `json:"event_type"`
	ActorID        string          `json:"actor_id"`
	OccurredAt     string          `json:"occurred_at"`
	Payload        json.RawMessage `json:"payload"`
	PreviousDigest string          `json:"previous_digest"`
	EventDigest    string          `json:"event_digest"`
	RequestID      string          `json:"request_id"`
}

func Build(c *domain.RemediationCase, events []domain.AuditEvent) ([]byte, string, error) {
	if c.Status != domain.StatusArchived || c.FrozenAt == nil || c.Review == nil || c.Review.Decision != "pass" {
		return nil, "", fmt.Errorf("仅通过复核的终局案件可生成归档")
	}
	baselineFrozen := ""
	if c.BaselineFrozenAt != nil {
		baselineFrozen = c.BaselineFrozenAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	frozen := c.FrozenAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	manifest := Manifest{ManifestVersion: ManifestVersion, CaseID: c.CaseID, TerminalRevision: c.Revision, Case: archiveCase{SiteName: c.SiteName, BuildingZone: c.BuildingZone, Status: c.Status, SurveyLeadID: c.SurveyLeadID, PlanApproverID: c.PlanApproverID, FieldLeadID: c.FieldLeadID, ReviewerID: c.ReviewerID, CreatedAt: c.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), BaselineFrozenAt: baselineFrozen, FrozenAt: frozen, Components: c.Components, Risk: c.Risk, Plan: c.Plan, PlanHistory: c.PlanHistory, PlanReturns: c.PlanReturns, Review: c.Review, ReviewHistory: c.ReviewHistory}}
	manifest.Events = make([]archiveEvent, len(events))
	for i, e := range events {
		manifest.Events[i] = archiveEvent{EventID: e.EventID, Sequence: e.Sequence, EventType: e.EventType, ActorID: e.ActorID, OccurredAt: e.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), Payload: json.RawMessage(e.Payload), PreviousDigest: e.PreviousDigest, EventDigest: e.EventDigest, RequestID: e.RequestID}
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}
