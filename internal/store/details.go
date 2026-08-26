package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"timber-pest-remediation-ledger/internal/domain"
)

func syncDetails(ctx context.Context, tx *sql.Tx, c *domain.RemediationCase) error {
	for _, statement := range []string{
		`DELETE FROM observations WHERE case_id=?`,
		`DELETE FROM execution_attempts WHERE case_id=?`,
		`DELETE FROM zones WHERE case_id=?`,
		`DELETE FROM components WHERE case_id=?`,
	} {
		if _, err := tx.ExecContext(ctx, statement, c.CaseID); err != nil {
			return err
		}
	}
	for _, component := range c.Components {
		_, err := tx.ExecContext(ctx, `INSERT INTO components(case_id,component_id,location_code,component_type,heritage_grade,pest_clue,activity_score,damage_extent,moisture,evidence_digest) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(case_id,component_id) DO UPDATE SET location_code=excluded.location_code,component_type=excluded.component_type,heritage_grade=excluded.heritage_grade,pest_clue=excluded.pest_clue,activity_score=excluded.activity_score,damage_extent=excluded.damage_extent,moisture=excluded.moisture,evidence_digest=excluded.evidence_digest`, c.CaseID, component.ComponentID, component.LocationCode, component.ComponentType, component.HeritageGrade, component.PestClue, component.ActivityScore, component.DamageExtentPercent, component.MoisturePercent, component.EvidenceDigest)
		if err != nil {
			return err
		}
	}
	if c.Plan == nil {
		return nil
	}
	for _, zone := range c.Plan.Zones {
		zoneJSON, err := json.Marshal(zone)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO zones(case_id,zone_id,method,execution_status,monitoring_status,attempt_number,zone_json) VALUES(?,?,?,?,?,?,?) ON CONFLICT(case_id,zone_id) DO UPDATE SET method=excluded.method,execution_status=excluded.execution_status,monitoring_status=excluded.monitoring_status,attempt_number=excluded.attempt_number,zone_json=excluded.zone_json`, c.CaseID, zone.ZoneID, zone.Method, zone.ExecutionStatus, zone.MonitoringStatus, zone.AttemptNumber, zoneJSON)
		if err != nil {
			return err
		}
		for _, attempt := range zone.Attempts {
			data, err := json.Marshal(attempt)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO execution_attempts(case_id,zone_id,attempt_number,executed_at,executed_by,evidence_digest,deviation_severity,attempt_json) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(case_id,zone_id,attempt_number) DO UPDATE SET attempt_json=excluded.attempt_json,deviation_severity=excluded.deviation_severity`, c.CaseID, zone.ZoneID, attempt.AttemptNumber, attempt.ExecutedAt.UTC().Format(time.RFC3339Nano), attempt.ExecutedBy, attempt.EvidenceDigest, attempt.DeviationSeverity, data)
			if err != nil {
				return err
			}
		}
		for _, observation := range zone.Observations {
			data, err := json.Marshal(observation)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO observations(case_id,zone_id,observation_id,attempt_number,observed_at,observation_json) VALUES(?,?,?,?,?,?) ON CONFLICT(case_id,observation_id) DO NOTHING`, c.CaseID, zone.ZoneID, observation.ObservationID, observation.AttemptNumber, observation.ObservedAt.UTC().Format(time.RFC3339Nano), data)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
