package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"timber-pest-remediation-ledger/internal/domain"
)

func (s *Store) VerifyIntegrity(ctx context.Context) error {
	if err := s.verifyIdempotencyCache(ctx); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT case_id,revision,status,archive_digest FROM cases ORDER BY case_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type state struct {
		id       string
		revision int64
		status   domain.CaseStatus
		digest   string
	}
	var states []state
	for rows.Next() {
		var item state
		if err := rows.Scan(&item.id, &item.revision, &item.status, &item.digest); err != nil {
			return err
		}
		states = append(states, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range states {
		aggregate, err := s.GetCase(ctx, item.id)
		if err != nil {
			return err
		}
		if err := domain.ValidateAggregate(aggregate); err != nil {
			return err
		}
		events, err := s.ListEvents(ctx, item.id)
		if err != nil {
			return err
		}
		if int64(len(events)) != item.revision {
			return fmt.Errorf("案件 %s 的 revision=%d 但事件数=%d", item.id, item.revision, len(events))
		}
		previous := ""
		for i, event := range events {
			if event.Sequence != int64(i+1) {
				return fmt.Errorf("案件 %s 的事件序号不连续", item.id)
			}
			if err := verifyEvent(previous, event); err != nil {
				return err
			}
			previous = event.EventDigest
		}
		if item.status == domain.StatusArchived {
			var revision int64
			var digest string
			err := s.db.QueryRowContext(ctx, `SELECT terminal_revision,digest FROM archives WHERE case_id=?`, item.id).Scan(&revision, &digest)
			if err == sql.ErrNoRows {
				return fmt.Errorf("已归档案件 %s 缺少归档记录", item.id)
			}
			if err != nil {
				return err
			}
			if revision != item.revision || digest == "" || digest != item.digest {
				return fmt.Errorf("案件 %s 的归档版本或摘要不一致", item.id)
			}
		}
		if err := s.verifyNormalizedFacts(ctx, item.id); err != nil {
			return err
		}
	}
	return nil
}

// verifyIdempotencyCache 拒绝启动时仍存在不可解码、结构非法或案件归属错误的
// 幂等缓存记录，避免服务以"通过"状态启动后让相同 request_id 的命令重放只能
// 返回 internal_error。
func (s *Store) verifyIdempotencyCache(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT case_id,request_id,fingerprint,response_json FROM idempotency ORDER BY case_id,request_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var caseID, requestID, fingerprint string
		var response []byte
		if err := rows.Scan(&caseID, &requestID, &fingerprint, &response); err != nil {
			return err
		}
		if fingerprint == "" {
			return domain.NewError(domain.KindCorrupt, "idempotency_corrupt", "案件 %s 的 request_id=%s 幂等记录缺少指纹", caseID, requestID)
		}
		var existing domain.RemediationCase
		if err := json.Unmarshal(response, &existing); err != nil {
			return domain.NewError(domain.KindCorrupt, "idempotency_corrupt", "案件 %s 的 request_id=%s 幂等响应无法解码: %v", caseID, requestID, err)
		}
		if err := domain.ValidateAggregate(&existing); err != nil {
			return domain.NewError(domain.KindCorrupt, "idempotency_corrupt", "案件 %s 的 request_id=%s 幂等响应聚合无效: %v", caseID, requestID, err)
		}
		if existing.CaseID != caseID {
			return domain.NewError(domain.KindCorrupt, "idempotency_corrupt", "案件 %s 的 request_id=%s 幂等响应归属案件 %s", caseID, requestID, existing.CaseID)
		}
	}
	return rows.Err()
}

func (s *Store) verifyNormalizedFacts(ctx context.Context, caseID string) error {
	c, err := s.GetCase(ctx, caseID)
	if err != nil {
		return err
	}
	var componentCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM components WHERE case_id=?`, caseID).Scan(&componentCount); err != nil {
		return err
	}
	if componentCount != len(c.Components) {
		return fmt.Errorf("案件 %s 的规范化构件数与聚合不一致", caseID)
	}
	for _, component := range c.Components {
		var digest, grade string
		var activity int
		var damage, moisture float64
		err := s.db.QueryRowContext(ctx, `SELECT evidence_digest,heritage_grade,activity_score,damage_extent,moisture FROM components WHERE case_id=? AND component_id=?`, caseID, component.ComponentID).Scan(&digest, &grade, &activity, &damage, &moisture)
		if err != nil {
			return fmt.Errorf("读取案件 %s 构件 %s: %w", caseID, component.ComponentID, err)
		}
		if digest != component.EvidenceDigest || grade != component.HeritageGrade || activity != component.ActivityScore || damage != component.DamageExtentPercent || moisture != component.MoisturePercent {
			return fmt.Errorf("案件 %s 构件 %s 的规范化事实不一致", caseID, component.ComponentID)
		}
	}
	if c.Plan == nil {
		return nil
	}
	var zoneCount, attemptCount, observationCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM zones WHERE case_id=?`, caseID).Scan(&zoneCount); err != nil {
		return err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_attempts WHERE case_id=?`, caseID).Scan(&attemptCount); err != nil {
		return err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM observations WHERE case_id=?`, caseID).Scan(&observationCount); err != nil {
		return err
	}
	expectedAttempts, expectedObservations := 0, 0
	if zoneCount != len(c.Plan.Zones) {
		return fmt.Errorf("案件 %s 的规范化分区数与当前方案不一致", caseID)
	}
	for _, zone := range c.Plan.Zones {
		expectedAttempts += len(zone.Attempts)
		expectedObservations += len(zone.Observations)
		var stored []byte
		if err := s.db.QueryRowContext(ctx, `SELECT zone_json FROM zones WHERE case_id=? AND zone_id=?`, caseID, zone.ZoneID).Scan(&stored); err != nil {
			return err
		}
		expected, err := json.Marshal(zone)
		if err != nil {
			return err
		}
		if string(stored) != string(expected) {
			return fmt.Errorf("案件 %s 分区 %s 的规范化快照不一致", caseID, zone.ZoneID)
		}
	}
	if attemptCount != expectedAttempts {
		return fmt.Errorf("案件 %s 的规范化施作尝试数不一致", caseID)
	}
	if observationCount != expectedObservations {
		return fmt.Errorf("案件 %s 的规范化监测数不一致", caseID)
	}
	return nil
}
