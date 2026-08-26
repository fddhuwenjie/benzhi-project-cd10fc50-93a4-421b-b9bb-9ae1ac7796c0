package cases

import (
	"time"

	"timber-pest-remediation-ledger/internal/domain"
)

type CommandMeta struct {
	RequestID        string `json:"request_id"`
	ActorID          string `json:"actor_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type CreateCommand struct {
	RequestID    string                   `json:"request_id"`
	ActorID      string                   `json:"actor_id"`
	CaseID       string                   `json:"case_id"`
	SiteName     string                   `json:"site_name"`
	BuildingZone string                   `json:"building_zone"`
	SurveyLeadID string                   `json:"survey_lead_id"`
	Components   []domain.TimberComponent `json:"components"`
}

type PlanCommand struct {
	CommandMeta
	Zones []domain.ZonePlanInput `json:"zones"`
}
type ComponentRevisionCommand struct {
	CommandMeta
	Operations []domain.ComponentRevisionOperation `json:"operations"`
}
type PlanReturnCommand struct {
	CommandMeta
	Reason  string   `json:"reason"`
	ZoneIDs []string `json:"zone_ids"`
}
type ExecutionCommand struct {
	CommandMeta
	Execution domain.ExecutionInput `json:"execution"`
}
type ExecutionBatchCommand struct {
	CommandMeta
	Executions []domain.ZoneExecutionInput `json:"executions"`
}
type RemediationCommand struct {
	CommandMeta
	Decision string `json:"decision"`
}
type ObservationCommand struct {
	CommandMeta
	Observation domain.ObservationInput `json:"observation"`
}
type ReviewCommand struct {
	CommandMeta
	Decision         string `json:"decision"`
	Findings         string `json:"findings"`
	EvidenceComplete bool   `json:"evidence_complete"`
}

type Clock interface{ Now() time.Time }
type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }
