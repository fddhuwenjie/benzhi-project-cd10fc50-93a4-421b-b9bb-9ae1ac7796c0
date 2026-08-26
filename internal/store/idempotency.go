package store

import (
	"context"
	"database/sql"
	"errors"

	"timber-pest-remediation-ledger/internal/domain"
)

func idempotencyCacheKey(caseID, requestID string) string {
	return caseID + "\x00" + requestID
}

func (s *Store) readCachedCommit(caseID, requestID, fingerprint string) (*CommitResult, bool, error) {
	s.idempotencyMu.RLock()
	entry, ok := s.idempotencyHot[idempotencyCacheKey(caseID, requestID)]
	s.idempotencyMu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if entry.fingerprint != fingerprint {
		return nil, false, domain.Conflict("idempotency_conflict", "相同 request_id 已用于不同请求")
	}
	result := *entry.result
	result.Replayed = true
	return &result, true, nil
}

func (s *Store) rememberCommit(caseID, requestID, fingerprint string, result *CommitResult) {
	s.idempotencyMu.Lock()
	s.idempotencyHot[idempotencyCacheKey(caseID, requestID)] = cachedCommit{fingerprint: fingerprint, result: result}
	s.idempotencyMu.Unlock()
}

func readIdempotency(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, caseID, requestID, fingerprint string) ([]byte, bool, error) {
	var storedFingerprint string
	var response []byte
	err := q.QueryRowContext(ctx, `SELECT fingerprint, response_json FROM idempotency WHERE case_id=? AND request_id=?`, caseID, requestID).Scan(&storedFingerprint, &response)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if storedFingerprint != fingerprint {
		return nil, false, domain.Conflict("idempotency_conflict", "相同 request_id 已用于不同请求")
	}
	return response, true, nil
}
