package domain

import (
	"sort"
	"strings"
	"time"
)

type MonitoringSummary struct {
	CaseID             string                   `json:"case_id"`
	Revision           int64                    `json:"revision"`
	Status             CaseStatus               `json:"status"`
	ReviewReady        bool                     `json:"review_ready"`
	PendingExecution   int                      `json:"pending_execution"`
	Collecting         int                      `json:"collecting"`
	ReadyToEvaluate    int                      `json:"ready_to_evaluate"`
	PendingRemediation int                      `json:"pending_remediation"`
	Passed             int                      `json:"passed"`
	Zones              []ZoneMonitoringProgress `json:"zones"`
}

type ZoneMonitoringProgress struct {
	ZoneID                string     `json:"zone_id"`
	AttemptNumber         int        `json:"attempt_number"`
	ExecutionStatus       string     `json:"execution_status"`
	MonitoringStatus      string     `json:"monitoring_status"`
	EarliestObservationAt *time.Time `json:"earliest_observation_at,omitempty"`
	ObservationCount      int        `json:"observation_count"`
	MinObservations       int        `json:"min_observations"`
	ExceededThresholds    []string   `json:"exceeded_thresholds"`
	BlockerCode           string     `json:"blocker_code"`
	NextAction            string     `json:"next_action"`
}

func BuildMonitoringSummary(c *RemediationCase, now time.Time) MonitoringSummary {
	result := MonitoringSummary{CaseID: c.CaseID, Revision: c.Revision, Status: c.Status, Zones: []ZoneMonitoringProgress{}}
	if c.Plan == nil {
		return result
	}
	// Build a private, per-call copy of the zones so concurrent summaries for
	// different cases never share or mutate state. Each summary owns its slice
	// independently, which keeps sorting stable and prevents cross-case
	// pollution regardless of how many requests run in parallel.
	zones := make([]TreatmentZone, len(c.Plan.Zones))
	copy(zones, c.Plan.Zones)
	sort.Slice(zones, func(i, j int) bool { return zones[i].ZoneID < zones[j].ZoneID })
	for index := range zones {
		zone := &zones[index]
		current := currentObservations(zone)
		progress := ZoneMonitoringProgress{ZoneID: zone.ZoneID, AttemptNumber: zone.AttemptNumber, ExecutionStatus: zone.ExecutionStatus, MonitoringStatus: zone.MonitoringStatus, ObservationCount: len(current), MinObservations: zone.Thresholds.MinObservations, ExceededThresholds: exceededThresholds(zone, current)}
		last := lastAttempt(zone)
		if last != nil {
			earliest := last.ExecutedAt.Add(time.Duration(zone.Thresholds.ObservationWindowHours) * time.Hour).UTC()
			progress.EarliestObservationAt = &earliest
		}
		switch {
		case zone.AttemptNumber == 0:
			progress.BlockerCode, progress.NextAction = "execution_required", "record_execution"
			result.PendingExecution++
		case zone.MonitoringStatus == "failed" || (last != nil && last.DeviationSeverity == "major"):
			if last != nil && last.RemediationDecision != "" {
				progress.BlockerCode, progress.NextAction = "execution_required", "record_execution"
				result.PendingExecution++
			} else {
				progress.BlockerCode, progress.NextAction = "remediation_required", "decide_remediation"
				result.PendingRemediation++
			}
		case zone.MonitoringStatus == "passed":
			progress.BlockerCode, progress.NextAction = "none", "request_review"
			result.Passed++
		case progress.EarliestObservationAt != nil && now.UTC().Before(*progress.EarliestObservationAt):
			progress.BlockerCode, progress.NextAction = "observation_window_pending", "wait_for_observation_window"
			result.Collecting++
		case len(current) < zone.Thresholds.MinObservations:
			progress.BlockerCode, progress.NextAction = "observations_insufficient", "record_observation"
			result.Collecting++
		default:
			progress.BlockerCode, progress.NextAction = "evaluation_required", "evaluate_zone"
			result.ReadyToEvaluate++
		}
		result.Zones = append(result.Zones, progress)
	}
	result.ReviewReady = len(result.Zones) > 0 && result.Passed == len(result.Zones)
	return result
}

func exceededThresholds(zone *TreatmentZone, observations []MonitoringObservation) []string {
	activity, acoustic, visual := false, false, false
	for _, observation := range observations {
		activity = activity || observation.ActivityCount > zone.Thresholds.MaxActivityCount
		acoustic = acoustic || observation.AcousticScore > zone.Thresholds.MaxAcousticScore
		visual = visual || (!zone.Thresholds.AllowVisualActivity && strings.TrimSpace(observation.VisualFinding) != "" && observation.VisualFinding != "none")
	}
	out := make([]string, 0, 3)
	if activity {
		out = append(out, "activity_count")
	}
	if acoustic {
		out = append(out, "acoustic_score")
	}
	if visual {
		out = append(out, "visual_finding")
	}
	return out
}
