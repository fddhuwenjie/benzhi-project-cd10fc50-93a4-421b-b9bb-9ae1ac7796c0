package store

import (
	"context"
	"database/sql"
	"errors"

	"timber-pest-remediation-ledger/internal/domain"
)

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
