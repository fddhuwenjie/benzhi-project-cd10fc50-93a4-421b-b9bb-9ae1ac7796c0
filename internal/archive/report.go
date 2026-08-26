package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"

	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

type ReportSource interface {
	Source
	ReadNormalizedFacts(context.Context, string) (*store.NormalizedFacts, error)
}

type CheckResult struct {
	CheckCode     string `json:"check_code"`
	Status        string `json:"status"`
	EventSequence int64  `json:"event_sequence,omitempty"`
	ZoneID        string `json:"zone_id,omitempty"`
	ComponentID   string `json:"component_id,omitempty"`
	Expected      string `json:"expected,omitempty"`
	Actual        string `json:"actual,omitempty"`
}

type VerificationReport struct {
	CaseID           string        `json:"case_id"`
	Valid            bool          `json:"valid"`
	TerminalRevision int64         `json:"terminal_revision"`
	Checks           []CheckResult `json:"checks"`
}

func BuildVerificationReport(ctx context.Context, source ReportSource, caseID string) (*VerificationReport, error) {
	c, err := source.GetCase(ctx, caseID)
	if err != nil {
		return nil, err
	}
	if c.Status != domain.StatusArchived || c.FrozenAt == nil {
		return nil, domain.Gate("archive_not_ready", "案件尚未归档")
	}
	events, err := source.ListEvents(ctx, caseID)
	if err != nil {
		return nil, err
	}
	facts, err := source.ReadNormalizedFacts(ctx, caseID)
	if err != nil {
		return nil, err
	}
	record, archiveErr := source.GetArchive(ctx, caseID)
	if archiveErr != nil {
		var business *domain.BusinessError
		if !errors.As(archiveErr, &business) || business.Kind != domain.KindNotFound {
			return nil, archiveErr
		}
		record = nil
	}
	report := &VerificationReport{CaseID: caseID, Valid: true, TerminalRevision: c.Revision, Checks: []CheckResult{}}
	appendCheck := func(check CheckResult, pass bool) {
		if pass {
			check.Status = "pass"
			check.Expected = ""
			check.Actual = ""
		} else {
			check.Status = "fail"
			report.Valid = false
		}
		report.Checks = append(report.Checks, check)
	}

	previousDigest := ""
	for index, event := range events {
		expectedSequence := int64(index + 1)
		appendCheck(CheckResult{CheckCode: "event_sequence", EventSequence: event.Sequence, Expected: strconv.FormatInt(expectedSequence, 10), Actual: strconv.FormatInt(event.Sequence, 10)}, event.Sequence == expectedSequence)
		appendCheck(CheckResult{CheckCode: "event_previous_digest", EventSequence: event.Sequence, Expected: previousDigest, Actual: event.PreviousDigest}, event.PreviousDigest == previousDigest)
		expectedDigest := auditDigest(event)
		selfValid := event.EventDigest == expectedDigest && len(expectedDigest) >= 24 && event.EventID == expectedDigest[:24]
		appendCheck(CheckResult{CheckCode: "event_sha256", EventSequence: event.Sequence, Expected: expectedDigest, Actual: event.EventDigest}, selfValid)
		previousDigest = event.EventDigest
	}
	appendCheck(CheckResult{CheckCode: "event_revision", Expected: strconv.FormatInt(c.Revision, 10), Actual: strconv.Itoa(len(events))}, int64(len(events)) == c.Revision)

	if record == nil {
		appendCheck(CheckResult{CheckCode: "archive_record", Expected: "present", Actual: "missing"}, false)
	} else {
		report.TerminalRevision = record.TerminalRevision
		appendCheck(CheckResult{CheckCode: "terminal_revision", Expected: strconv.FormatInt(c.Revision, 10), Actual: strconv.FormatInt(record.TerminalRevision, 10)}, record.TerminalRevision == c.Revision)
		appendCheck(CheckResult{CheckCode: "aggregate_archive_digest", Expected: record.Digest, Actual: c.ArchiveDigest}, c.ArchiveDigest == record.Digest)
		rebuilt, digest, buildErr := Build(c, events)
		if buildErr != nil {
			appendCheck(CheckResult{CheckCode: "manifest_rebuild", Expected: "buildable", Actual: "failed"}, false)
		} else {
			appendCheck(CheckResult{CheckCode: "rebuilt_manifest_digest", Expected: record.Digest, Actual: digest}, digest == record.Digest)
			appendCheck(CheckResult{CheckCode: "manifest_bytes", Expected: digestBytes(rebuilt), Actual: digestBytes(record.Manifest)}, bytes.Equal(rebuilt, record.Manifest))
		}
	}
	checkNormalized(report, facts, c, appendCheck)
	return report, nil
}

func auditDigest(event domain.AuditEvent) string {
	h := sha256.New()
	timestamp := event.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	for _, part := range []string{event.PreviousDigest, event.CaseID, strconv.FormatInt(event.Sequence, 10), event.EventType, event.ActorID, timestamp, string(event.Payload), event.RequestID} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func checkNormalized(report *VerificationReport, facts *store.NormalizedFacts, c *domain.RemediationCase, appendCheck func(CheckResult, bool)) {
	appendCheck(CheckResult{CheckCode: "normalized_component_count", Expected: strconv.Itoa(len(c.Components)), Actual: strconv.Itoa(len(facts.Components))}, len(c.Components) == len(facts.Components))
	expectedComponents := make(map[string]domain.TimberComponent, len(c.Components))
	for _, component := range c.Components {
		expectedComponents[component.ComponentID] = component
	}
	for _, actual := range facts.Components {
		expected, ok := expectedComponents[actual.ComponentID]
		expectedDigest := "absent"
		if ok {
			expectedDigest = digestValue(struct {
				Grade            string
				Activity         int
				Damage, Moisture float64
				Evidence         string
			}{expected.HeritageGrade, expected.ActivityScore, expected.DamageExtentPercent, expected.MoisturePercent, expected.EvidenceDigest})
		}
		actualDigest := digestValue(struct {
			Grade            string
			Activity         int
			Damage, Moisture float64
			Evidence         string
		}{actual.HeritageGrade, actual.ActivityScore, actual.DamageExtent, actual.Moisture, actual.EvidenceDigest})
		appendCheck(CheckResult{CheckCode: "normalized_component_fact", ComponentID: actual.ComponentID, Expected: expectedDigest, Actual: actualDigest}, ok && expectedDigest == actualDigest)
		delete(expectedComponents, actual.ComponentID)
	}
	for _, componentID := range sortedKeys(expectedComponents) {
		appendCheck(CheckResult{CheckCode: "normalized_component_fact", ComponentID: componentID, Expected: "present", Actual: "missing"}, false)
	}

	expectedZones := make(map[string]domain.TreatmentZone)
	expectedExecutions := make(map[string]domain.ExecutionAttempt)
	expectedObservations := make(map[string]domain.MonitoringObservation)
	if c.Plan != nil {
		for _, zone := range c.Plan.Zones {
			expectedZones[zone.ZoneID] = zone
			for _, attempt := range zone.Attempts {
				expectedExecutions[zone.ZoneID+"\x00"+strconv.Itoa(attempt.AttemptNumber)] = attempt
			}
			for _, observation := range zone.Observations {
				expectedObservations[zone.ZoneID+"\x00"+observation.ObservationID] = observation
			}
		}
	}
	appendCheck(CheckResult{CheckCode: "normalized_zone_count", Expected: strconv.Itoa(len(expectedZones)), Actual: strconv.Itoa(len(facts.Zones))}, len(expectedZones) == len(facts.Zones))
	for _, actual := range facts.Zones {
		expected, ok := expectedZones[actual.ZoneID]
		expectedBytes, _ := json.Marshal(expected)
		appendCheck(CheckResult{CheckCode: "normalized_zone_snapshot", ZoneID: actual.ZoneID, Expected: digestBytes(expectedBytes), Actual: digestBytes(actual.Snapshot)}, ok && bytes.Equal(expectedBytes, actual.Snapshot))
		delete(expectedZones, actual.ZoneID)
	}
	for _, zoneID := range sortedKeys(expectedZones) {
		appendCheck(CheckResult{CheckCode: "normalized_zone_snapshot", ZoneID: zoneID, Expected: "present", Actual: "missing"}, false)
	}

	appendCheck(CheckResult{CheckCode: "normalized_execution_count", Expected: strconv.Itoa(len(expectedExecutions)), Actual: strconv.Itoa(len(facts.Executions))}, len(expectedExecutions) == len(facts.Executions))
	for _, actual := range facts.Executions {
		key := actual.ZoneID + "\x00" + strconv.Itoa(actual.AttemptNumber)
		expected, ok := expectedExecutions[key]
		expectedBytes, _ := json.Marshal(expected)
		valid := ok && expected.EvidenceDigest == actual.EvidenceDigest && bytes.Equal(expectedBytes, actual.Snapshot)
		appendCheck(CheckResult{CheckCode: "normalized_execution_fact", ZoneID: actual.ZoneID, Expected: digestBytes(expectedBytes), Actual: digestBytes(actual.Snapshot)}, valid)
		delete(expectedExecutions, key)
	}
	for _, key := range sortedKeys(expectedExecutions) {
		zoneID := key[:bytes.IndexByte([]byte(key), 0)]
		appendCheck(CheckResult{CheckCode: "normalized_execution_fact", ZoneID: zoneID, Expected: "present", Actual: "missing"}, false)
	}

	appendCheck(CheckResult{CheckCode: "normalized_monitoring_count", Expected: strconv.Itoa(len(expectedObservations)), Actual: strconv.Itoa(len(facts.Observations))}, len(expectedObservations) == len(facts.Observations))
	for _, actual := range facts.Observations {
		key := actual.ZoneID + "\x00" + actual.ObservationID
		expected, ok := expectedObservations[key]
		expectedBytes, _ := json.Marshal(expected)
		valid := ok && expected.AttemptNumber == actual.AttemptNumber && bytes.Equal(expectedBytes, actual.Snapshot)
		appendCheck(CheckResult{CheckCode: "normalized_monitoring_fact", ZoneID: actual.ZoneID, Expected: digestBytes(expectedBytes), Actual: digestBytes(actual.Snapshot)}, valid)
		delete(expectedObservations, key)
	}
	for _, key := range sortedKeys(expectedObservations) {
		zoneID := key[:bytes.IndexByte([]byte(key), 0)]
		appendCheck(CheckResult{CheckCode: "normalized_monitoring_fact", ZoneID: zoneID, Expected: "present", Actual: "missing"}, false)
	}
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func digestValue(value any) string {
	data, _ := json.Marshal(value)
	return digestBytes(data)
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
