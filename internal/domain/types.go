package domain

import "time"

type CaseStatus string

const (
	StatusDraft               CaseStatus = "draft"
	StatusBaselineFrozen      CaseStatus = "baseline_frozen"
	StatusRiskAssessed        CaseStatus = "risk_assessed"
	StatusPlanSubmitted       CaseStatus = "plan_submitted"
	StatusPlanReturned        CaseStatus = "plan_returned"
	StatusPlanApproved        CaseStatus = "plan_approved"
	StatusInExecution         CaseStatus = "in_execution"
	StatusSuspended           CaseStatus = "suspended"
	StatusMonitoring          CaseStatus = "monitoring"
	StatusRemediationRequired CaseStatus = "remediation_required"
	StatusReadyForReview      CaseStatus = "ready_for_review"
	StatusReviewReturned      CaseStatus = "review_returned"
	StatusArchived            CaseStatus = "archived"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskModerate RiskLevel = "moderate"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type RemediationCase struct {
	CaseID           string               `json:"case_id"`
	SiteName         string               `json:"site_name"`
	BuildingZone     string               `json:"building_zone"`
	Status           CaseStatus           `json:"status"`
	Revision         int64                `json:"revision"`
	SurveyLeadID     string               `json:"survey_lead_id"`
	PlanApproverID   string               `json:"plan_approver_id,omitempty"`
	FieldLeadID      string               `json:"field_lead_id,omitempty"`
	ReviewerID       string               `json:"reviewer_id,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	BaselineFrozenAt *time.Time           `json:"baseline_frozen_at,omitempty"`
	FrozenAt         *time.Time           `json:"frozen_at,omitempty"`
	Components       []TimberComponent    `json:"components"`
	Risk             *RiskAssessment      `json:"risk,omitempty"`
	Plan             *TreatmentPlan       `json:"plan,omitempty"`
	PlanHistory      []TreatmentPlan      `json:"plan_history,omitempty"`
	PlanReturns      []PlanReturnDecision `json:"plan_returns,omitempty"`
	Review           *ReviewDecision      `json:"review,omitempty"`
	ReviewHistory    []ReviewDecision     `json:"review_history,omitempty"`
	ArchiveDigest    string               `json:"archive_digest,omitempty"`
}

type TimberComponent struct {
	ComponentID         string  `json:"component_id"`
	LocationCode        string  `json:"location_code"`
	ComponentType       string  `json:"component_type"`
	HeritageGrade       string  `json:"heritage_grade"`
	PestClue            string  `json:"pest_clue"`
	ActivityScore       int     `json:"activity_score"`
	DamageExtentPercent float64 `json:"damage_extent_percent"`
	MoisturePercent     float64 `json:"moisture_percent"`
	EvidenceDigest      string  `json:"evidence_digest"`
}

type RiskAssessment struct {
	RuleVersion string       `json:"rule_version"`
	Score       int          `json:"score"`
	Level       RiskLevel    `json:"level"`
	Factors     []RiskFactor `json:"factors"`
	AssessedAt  time.Time    `json:"assessed_at"`
	AssessedBy  string       `json:"assessed_by"`
}

type RiskFactor struct {
	ComponentID string `json:"component_id"`
	Name        string `json:"name"`
	Points      int    `json:"points"`
	Basis       string `json:"basis"`
}

type TreatmentPlan struct {
	Version     int             `json:"version"`
	SubmittedBy string          `json:"submitted_by"`
	SubmittedAt time.Time       `json:"submitted_at"`
	ApprovedBy  string          `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time      `json:"approved_at,omitempty"`
	Zones       []TreatmentZone `json:"zones"`
}

type PlanReturnDecision struct {
	PlanVersion int       `json:"plan_version"`
	ReturnedBy  string    `json:"returned_by"`
	ReturnedAt  time.Time `json:"returned_at"`
	Reason      string    `json:"reason"`
	ZoneIDs     []string  `json:"zone_ids"`
}

type TreatmentZone struct {
	ZoneID                string                  `json:"zone_id"`
	ComponentIDs          []string                `json:"component_ids"`
	Method                string                  `json:"method"`
	ApprovedParameters    map[string]float64      `json:"approved_parameters"`
	ProtectionConstraints []string                `json:"protection_constraints"`
	ResponsibleID         string                  `json:"responsible_id"`
	Thresholds            AcceptanceThresholds    `json:"acceptance_thresholds"`
	ExecutionStatus       string                  `json:"execution_status"`
	DeviationSeverity     string                  `json:"deviation_severity,omitempty"`
	MonitoringStatus      string                  `json:"monitoring_status"`
	AttemptNumber         int                     `json:"attempt_number"`
	Attempts              []ExecutionAttempt      `json:"attempts"`
	Observations          []MonitoringObservation `json:"observations"`
}

type AcceptanceThresholds struct {
	MaxActivityCount       int     `json:"max_activity_count"`
	MaxAcousticScore       float64 `json:"max_acoustic_score"`
	AllowVisualActivity    bool    `json:"allow_visual_activity"`
	MinObservations        int     `json:"min_observations"`
	ObservationWindowHours int     `json:"observation_window_hours"`
}

type ExecutionAttempt struct {
	AttemptNumber       int                `json:"attempt_number"`
	ExecutedAt          time.Time          `json:"executed_at"`
	ExecutedBy          string             `json:"executed_by"`
	ActualParameters    map[string]float64 `json:"actual_parameters"`
	EvidenceDigest      string             `json:"evidence_digest"`
	DeviationSeverity   string             `json:"deviation_severity"`
	DeviationNote       string             `json:"deviation_note,omitempty"`
	RemediationDecision string             `json:"remediation_decision,omitempty"`
}

type MonitoringObservation struct {
	ObservationID  string    `json:"observation_id"`
	ObservedAt     time.Time `json:"observed_at"`
	Method         string    `json:"method"`
	ActivityCount  int       `json:"activity_count"`
	AcousticScore  float64   `json:"acoustic_score"`
	VisualFinding  string    `json:"visual_finding"`
	EvidenceDigest string    `json:"evidence_digest"`
	RecordedBy     string    `json:"recorded_by"`
	AttemptNumber  int       `json:"attempt_number"`
}

type ReviewDecision struct {
	ReviewerID       string    `json:"reviewer_id"`
	Decision         string    `json:"decision"`
	Findings         string    `json:"findings"`
	EvidenceComplete bool      `json:"evidence_complete"`
	DecidedAt        time.Time `json:"decided_at"`
}

type EventDraft struct {
	Type    string `json:"type"`
	ActorID string `json:"actor_id"`
	Payload any    `json:"payload"`
}

type AuditEvent struct {
	EventID        string    `json:"event_id"`
	CaseID         string    `json:"case_id"`
	Sequence       int64     `json:"sequence"`
	EventType      string    `json:"event_type"`
	ActorID        string    `json:"actor_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	Payload        []byte    `json:"payload"`
	PreviousDigest string    `json:"previous_digest"`
	EventDigest    string    `json:"event_digest"`
	RequestID      string    `json:"request_id"`
}
