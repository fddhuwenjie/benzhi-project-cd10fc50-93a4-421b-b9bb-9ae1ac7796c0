package store

import (
	"context"
	"database/sql"
	"encoding/json"
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

// decodeIdempotentCase 解码持久化的幂等响应并校验其结构与案件归属，确保重放返回
// 的是合法聚合而非损坏缓存。启动完整性检查已拒绝此类记录，但运行时仍保留防御，
// 将损坏映射为 integrity_error 而非 internal_error。
func decodeIdempotentCase(caseID, requestID string, response []byte) (*domain.RemediationCase, error) {
	var existing domain.RemediationCase
	if err := json.Unmarshal(response, &existing); err != nil {
		return nil, domain.NewError(domain.KindCorrupt, "idempotency_corrupt", "案件 %s 的 request_id=%s 幂等响应无法解码: %v", caseID, requestID, err)
	}
	if err := domain.ValidateAggregate(&existing); err != nil {
		return nil, domain.NewError(domain.KindCorrupt, "idempotency_corrupt", "案件 %s 的 request_id=%s 幂等响应聚合无效: %v", caseID, requestID, err)
	}
	if existing.CaseID != caseID {
		return nil, domain.NewError(domain.KindCorrupt, "idempotency_corrupt", "案件 %s 的 request_id=%s 幂等响应归属案件 %s", caseID, requestID, existing.CaseID)
	}
	return &existing, nil
}
