package store

import (
	"context"
	"fmt"
)

const schemaVersion = 1

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS cases (
			case_id TEXT PRIMARY KEY, status TEXT NOT NULL, revision INTEGER NOT NULL,
			aggregate_json BLOB NOT NULL, archive_digest TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS components (
			case_id TEXT NOT NULL, component_id TEXT NOT NULL, location_code TEXT NOT NULL,
			component_type TEXT NOT NULL, heritage_grade TEXT NOT NULL, pest_clue TEXT NOT NULL,
			activity_score INTEGER NOT NULL, damage_extent REAL NOT NULL, moisture REAL NOT NULL,
			evidence_digest TEXT NOT NULL, PRIMARY KEY(case_id, component_id),
			FOREIGN KEY(case_id) REFERENCES cases(case_id)
		)`,
		`CREATE TABLE IF NOT EXISTS zones (
			case_id TEXT NOT NULL, zone_id TEXT NOT NULL, method TEXT NOT NULL,
			execution_status TEXT NOT NULL, monitoring_status TEXT NOT NULL,
			attempt_number INTEGER NOT NULL, zone_json BLOB NOT NULL,
			PRIMARY KEY(case_id, zone_id), FOREIGN KEY(case_id) REFERENCES cases(case_id)
		)`,
		`CREATE TABLE IF NOT EXISTS observations (
			case_id TEXT NOT NULL, zone_id TEXT NOT NULL, observation_id TEXT NOT NULL,
			attempt_number INTEGER NOT NULL, observed_at TEXT NOT NULL, observation_json BLOB NOT NULL,
			PRIMARY KEY(case_id, observation_id), FOREIGN KEY(case_id, zone_id) REFERENCES zones(case_id, zone_id)
		)`,
		`CREATE TABLE IF NOT EXISTS execution_attempts (
			case_id TEXT NOT NULL, zone_id TEXT NOT NULL, attempt_number INTEGER NOT NULL,
			executed_at TEXT NOT NULL, executed_by TEXT NOT NULL, evidence_digest TEXT NOT NULL,
			deviation_severity TEXT NOT NULL, attempt_json BLOB NOT NULL,
			PRIMARY KEY(case_id, zone_id, attempt_number),
			FOREIGN KEY(case_id, zone_id) REFERENCES zones(case_id, zone_id)
		)`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			case_id TEXT NOT NULL, sequence INTEGER NOT NULL, event_id TEXT NOT NULL UNIQUE,
			event_type TEXT NOT NULL, actor_id TEXT NOT NULL, occurred_at TEXT NOT NULL,
			payload BLOB NOT NULL, previous_digest TEXT NOT NULL, event_digest TEXT NOT NULL,
			request_id TEXT NOT NULL, PRIMARY KEY(case_id, sequence),
			FOREIGN KEY(case_id) REFERENCES cases(case_id)
		)`,
		`CREATE TABLE IF NOT EXISTS idempotency (
			case_id TEXT NOT NULL, request_id TEXT NOT NULL, fingerprint TEXT NOT NULL,
			response_json BLOB NOT NULL, created_at TEXT NOT NULL,
			PRIMARY KEY(case_id, request_id)
		)`,
		`CREATE TABLE IF NOT EXISTS archives (
			case_id TEXT PRIMARY KEY, terminal_revision INTEGER NOT NULL,
			manifest_json BLOB NOT NULL, digest TEXT NOT NULL, created_at TEXT NOT NULL,
			FOREIGN KEY(case_id) REFERENCES cases(case_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_case ON audit_events(case_id, sequence)`,
		`CREATE INDEX IF NOT EXISTS idx_observations_zone ON observations(case_id, zone_id, attempt_number)`,
		`CREATE INDEX IF NOT EXISTS idx_attempts_zone ON execution_attempts(case_id, zone_id, attempt_number)`,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("执行模式迁移: %w", err)
		}
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_meta`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_meta(version) VALUES(?)`, schemaVersion); err != nil {
			return err
		}
	}
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT version FROM schema_meta LIMIT 1`).Scan(&version); err != nil {
		return err
	}
	if version != schemaVersion {
		return fmt.Errorf("不支持的数据库模式版本 %d", version)
	}
	return tx.Commit()
}
