package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"timber-pest-remediation-ledger/internal/domain"
)

type ArchiveRecord struct {
	CaseID           string
	TerminalRevision int64
	Manifest         []byte
	Digest           string
	CreatedAt        time.Time
}

func (s *Store) GetCase(ctx context.Context, caseID string) (*domain.RemediationCase, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT aggregate_json FROM cases WHERE case_id=?`, caseID).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NotFound("案件", caseID)
	}
	if err != nil {
		return nil, err
	}
	var c domain.RemediationCase
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, domain.NewError(domain.KindCorrupt, "aggregate_invalid", "案件聚合无法解码: %v", err)
	}
	return &c, nil
}

func (s *Store) ListEvents(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_id,sequence,event_type,actor_id,occurred_at,payload,previous_digest,event_digest,request_id FROM audit_events WHERE case_id=? ORDER BY sequence`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows, caseID)
}

func listEventsTx(ctx context.Context, tx *sql.Tx, caseID string) ([]domain.AuditEvent, error) {
	rows, err := tx.QueryContext(ctx, `SELECT event_id,sequence,event_type,actor_id,occurred_at,payload,previous_digest,event_digest,request_id FROM audit_events WHERE case_id=? ORDER BY sequence`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows, caseID)
}

type rowScanner interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanEvents(rows rowScanner, caseID string) ([]domain.AuditEvent, error) {
	var events []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		var occurred string
		if err := rows.Scan(&e.EventID, &e.Sequence, &e.EventType, &e.ActorID, &occurred, &e.Payload, &e.PreviousDigest, &e.EventDigest, &e.RequestID); err != nil {
			return nil, err
		}
		e.CaseID = caseID
		parsed, err := time.Parse(time.RFC3339Nano, occurred)
		if err != nil {
			return nil, err
		}
		e.OccurredAt = parsed
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) GetArchive(ctx context.Context, caseID string) (*ArchiveRecord, error) {
	var record ArchiveRecord
	var created string
	err := s.db.QueryRowContext(ctx, `SELECT case_id,terminal_revision,manifest_json,digest,created_at FROM archives WHERE case_id=?`, caseID).Scan(&record.CaseID, &record.TerminalRevision, &record.Manifest, &record.Digest, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NotFound("归档", caseID)
	}
	if err != nil {
		return nil, err
	}
	record.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return nil, err
	}
	return &record, nil
}
