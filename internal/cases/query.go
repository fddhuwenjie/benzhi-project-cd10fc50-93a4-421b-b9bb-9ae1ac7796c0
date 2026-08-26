package cases

import (
	"context"
	"fmt"

	"timber-pest-remediation-ledger/internal/archive"
	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

func (s *Service) Get(ctx context.Context, caseID string) (*domain.RemediationCase, error) {
	c, err := s.store.GetCase(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("查询案件 %s: %v", caseID, err)
	}
	return c, nil
}
func (s *Service) MonitoringSummary(ctx context.Context, caseID string) (*domain.MonitoringSummary, error) {
	c, err := s.store.GetCase(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("读取监测摘要案件 %s: %v", caseID, err)
	}
	result := domain.BuildMonitoringSummary(c, s.clock.Now())
	return &result, nil
}
func (s *Service) Timeline(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	if _, err := s.store.GetCase(ctx, caseID); err != nil {
		return nil, fmt.Errorf("读取时间线案件 %s: %v", caseID, err)
	}
	events, err := s.store.ListEvents(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("读取时间线事件 %s: %w", caseID, err)
	}
	return events, nil
}
func (s *Service) Archive(ctx context.Context, caseID string) (*store.ArchiveRecord, error) {
	record, err := s.store.GetArchive(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("读取归档 %s: %v", caseID, err)
	}
	return record, nil
}
func (s *Service) VerifyArchive(ctx context.Context, caseID string) (*archive.Verification, error) {
	verification, err := archive.Verify(ctx, s.store, caseID)
	if err != nil {
		return nil, fmt.Errorf("校验归档 %s: %v", caseID, err)
	}
	return verification, nil
}
func (s *Service) ArchiveVerificationReport(ctx context.Context, caseID string) (*archive.VerificationReport, error) {
	report, err := archive.BuildVerificationReport(ctx, s.store, caseID)
	if err != nil {
		return nil, fmt.Errorf("生成归档校验报告 %s: %v", caseID, err)
	}
	return report, nil
}
func (s *Service) Healthy(ctx context.Context) error {
	if err := s.store.Ping(ctx); err != nil {
		return fmt.Errorf("检查存储健康状态: %w", err)
	}
	return nil
}
