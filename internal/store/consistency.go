package store

import (
	"context"
)

type NormalizedComponent struct {
	ComponentID    string
	HeritageGrade  string
	ActivityScore  int
	DamageExtent   float64
	Moisture       float64
	EvidenceDigest string
}

type NormalizedZone struct {
	ZoneID   string
	Snapshot []byte
}

type NormalizedExecution struct {
	ZoneID         string
	AttemptNumber  int
	EvidenceDigest string
	Snapshot       []byte
}

type NormalizedObservation struct {
	ZoneID        string
	ObservationID string
	AttemptNumber int
	Snapshot      []byte
}

type NormalizedFacts struct {
	Components   []NormalizedComponent
	Zones        []NormalizedZone
	Executions   []NormalizedExecution
	Observations []NormalizedObservation
}

func (s *Store) ReadNormalizedFacts(ctx context.Context, caseID string) (*NormalizedFacts, error) {
	facts := &NormalizedFacts{}
	componentRows, err := s.db.QueryContext(ctx, `SELECT component_id,heritage_grade,activity_score,damage_extent,moisture,evidence_digest FROM components WHERE case_id=? ORDER BY component_id`, caseID)
	if err != nil {
		return nil, err
	}
	for componentRows.Next() {
		var item NormalizedComponent
		if err := componentRows.Scan(&item.ComponentID, &item.HeritageGrade, &item.ActivityScore, &item.DamageExtent, &item.Moisture, &item.EvidenceDigest); err != nil {
			componentRows.Close()
			return nil, err
		}
		facts.Components = append(facts.Components, item)
	}
	if err := componentRows.Err(); err != nil {
		componentRows.Close()
		return nil, err
	}
	componentRows.Close()

	zoneRows, err := s.db.QueryContext(ctx, `SELECT zone_id,zone_json FROM zones WHERE case_id=? ORDER BY zone_id`, caseID)
	if err != nil {
		return nil, err
	}
	for zoneRows.Next() {
		var item NormalizedZone
		if err := zoneRows.Scan(&item.ZoneID, &item.Snapshot); err != nil {
			zoneRows.Close()
			return nil, err
		}
		facts.Zones = append(facts.Zones, item)
	}
	if err := zoneRows.Err(); err != nil {
		zoneRows.Close()
		return nil, err
	}
	zoneRows.Close()

	executionRows, err := s.db.QueryContext(ctx, `SELECT zone_id,attempt_number,evidence_digest,attempt_json FROM execution_attempts WHERE case_id=? ORDER BY zone_id,attempt_number`, caseID)
	if err != nil {
		return nil, err
	}
	for executionRows.Next() {
		var item NormalizedExecution
		if err := executionRows.Scan(&item.ZoneID, &item.AttemptNumber, &item.EvidenceDigest, &item.Snapshot); err != nil {
			executionRows.Close()
			return nil, err
		}
		facts.Executions = append(facts.Executions, item)
	}
	if err := executionRows.Err(); err != nil {
		executionRows.Close()
		return nil, err
	}
	executionRows.Close()

	observationRows, err := s.db.QueryContext(ctx, `SELECT zone_id,observation_id,attempt_number,observation_json FROM observations WHERE case_id=? ORDER BY zone_id,observation_id`, caseID)
	if err != nil {
		return nil, err
	}
	for observationRows.Next() {
		var item NormalizedObservation
		if err := observationRows.Scan(&item.ZoneID, &item.ObservationID, &item.AttemptNumber, &item.Snapshot); err != nil {
			observationRows.Close()
			return nil, err
		}
		facts.Observations = append(facts.Observations, item)
	}
	if err := observationRows.Err(); err != nil {
		observationRows.Close()
		return nil, err
	}
	observationRows.Close()
	return facts, nil
}
