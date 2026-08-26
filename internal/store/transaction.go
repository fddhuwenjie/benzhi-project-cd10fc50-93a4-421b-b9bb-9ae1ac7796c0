package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"timber-pest-remediation-ledger/internal/domain"
)

type Mutation func(*domain.RemediationCase) (domain.EventDraft, error)
type ArchiveBuilder func(*domain.RemediationCase, []domain.AuditEvent) ([]byte, string, error)

type CommitResult struct {
	Case     *domain.RemediationCase
	Replayed bool
	Archive  []byte
}

func (s *Store) Create(ctx context.Context, c *domain.RemediationCase, draft domain.EventDraft, requestID, fingerprint string) (*CommitResult, error) {
	if err := domain.ValidateAggregate(c); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if response, ok, err := readIdempotency(ctx, tx, c.CaseID, requestID, fingerprint); err != nil {
		return nil, err
	} else if ok {
		var existing domain.RemediationCase
		if err := json.Unmarshal(response, &existing); err != nil {
			return nil, err
		}
		return &CommitResult{Case: &existing, Replayed: true}, nil
	}
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cases WHERE case_id=?`, c.CaseID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists != 0 {
		return nil, domain.Conflict("case_exists", "case_id 已存在")
	}
	now := time.Now().UTC()
	event, err := buildAuditEvent(c.CaseID, 1, "", requestID, draft, now)
	if err != nil {
		return nil, err
	}
	aggregate, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cases(case_id,status,revision,aggregate_json,archive_digest,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, c.CaseID, c.Status, c.Revision, aggregate, "", c.CreatedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return nil, err
	}
	if err := syncDetails(ctx, tx, c); err != nil {
		return nil, err
	}
	if err := insertEvent(ctx, tx, event); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO idempotency(case_id,request_id,fingerprint,response_json,created_at) VALUES(?,?,?,?,?)`, c.CaseID, requestID, fingerprint, aggregate, now.Format(time.RFC3339Nano)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &CommitResult{Case: c}, nil
}

func (s *Store) Commit(ctx context.Context, caseID string, expected int64, requestID, fingerprint string, mutate Mutation, builder ArchiveBuilder) (*CommitResult, error) {
	if cached, ok, err := s.readCachedCommit(caseID, requestID, fingerprint); err != nil || ok {
		return cached, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if response, ok, err := readIdempotency(ctx, tx, caseID, requestID, fingerprint); err != nil {
		return nil, err
	} else if ok {
		var existing domain.RemediationCase
		if err := json.Unmarshal(response, &existing); err != nil {
			return nil, err
		}
		archive, _ := readArchiveTx(ctx, tx, caseID)
		result := &CommitResult{Case: &existing, Replayed: true, Archive: archive}
		s.rememberCommit(caseID, requestID, fingerprint, result)
		return result, nil
	}
	c, err := loadCaseTx(ctx, tx, caseID)
	if err != nil {
		return nil, err
	}
	if c.Revision != expected {
		return nil, domain.Conflict("revision_conflict", "expected_revision=%d 与当前 revision=%d 不一致", expected, c.Revision)
	}
	draft, err := mutate(c)
	if err != nil {
		return nil, err
	}
	c.Revision++
	if err := domain.ValidateAggregate(c); err != nil {
		return nil, err
	}
	previous, sequence, err := lastEventState(ctx, tx, caseID)
	if err != nil {
		return nil, err
	}
	event, err := buildAuditEvent(caseID, sequence+1, previous, requestID, draft, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if err := insertEvent(ctx, tx, event); err != nil {
		return nil, err
	}
	var archiveJSON []byte
	if builder != nil {
		events, err := listEventsTx(ctx, tx, caseID)
		if err != nil {
			return nil, err
		}
		archiveJSON, c.ArchiveDigest, err = builder(c, events)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO archives(case_id,terminal_revision,manifest_json,digest,created_at) VALUES(?,?,?,?,?)`, caseID, c.Revision, archiveJSON, c.ArchiveDigest, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return nil, err
		}
	}
	aggregate, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE cases SET status=?,revision=?,aggregate_json=?,archive_digest=?,updated_at=? WHERE case_id=? AND revision=?`, c.Status, c.Revision, aggregate, c.ArchiveDigest, time.Now().UTC().Format(time.RFC3339Nano), caseID, expected)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, domain.Conflict("revision_conflict", "案件被并发修改")
	}
	if err := syncDetails(ctx, tx, c); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO idempotency(case_id,request_id,fingerprint,response_json,created_at) VALUES(?,?,?,?,?)`, caseID, requestID, fingerprint, aggregate, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	commitResult := &CommitResult{Case: c, Archive: archiveJSON}
	s.rememberCommit(caseID, requestID, fingerprint, commitResult)
	return commitResult, nil
}

func loadCaseTx(ctx context.Context, tx *sql.Tx, caseID string) (*domain.RemediationCase, error) {
	var data []byte
	err := tx.QueryRowContext(ctx, `SELECT aggregate_json FROM cases WHERE case_id=?`, caseID).Scan(&data)
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

func insertEvent(ctx context.Context, tx *sql.Tx, e domain.AuditEvent) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_events(case_id,sequence,event_id,event_type,actor_id,occurred_at,payload,previous_digest,event_digest,request_id) VALUES(?,?,?,?,?,?,?,?,?,?)`, e.CaseID, e.Sequence, e.EventID, e.EventType, e.ActorID, e.OccurredAt.Format(time.RFC3339Nano), e.Payload, e.PreviousDigest, e.EventDigest, e.RequestID)
	return err
}

func lastEventState(ctx context.Context, tx *sql.Tx, caseID string) (string, int64, error) {
	var digest string
	var sequence int64
	err := tx.QueryRowContext(ctx, `SELECT event_digest,sequence FROM audit_events WHERE case_id=? ORDER BY sequence DESC LIMIT 1`, caseID).Scan(&digest, &sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	return digest, sequence, err
}

func readArchiveTx(ctx context.Context, tx *sql.Tx, caseID string) ([]byte, error) {
	var data []byte
	err := tx.QueryRowContext(ctx, `SELECT manifest_json FROM archives WHERE case_id=?`, caseID).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return data, err
}

func wrapStore(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
