package cases

import (
	"context"

	"timber-pest-remediation-ledger/internal/archive"
	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

func (s *Service) Get(ctx context.Context, caseID string) (*domain.RemediationCase, error) {
	return s.store.GetCase(ctx, caseID)
}
func (s *Service) MonitoringSummary(ctx context.Context, caseID string) (*domain.MonitoringSummary, error) {
	c, err := s.store.GetCase(ctx, caseID)
	if err != nil {
		return nil, err
	}
	result := domain.BuildMonitoringSummary(c, s.clock.Now())
	return &result, nil
}
func (s *Service) Timeline(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	if _, err := s.store.GetCase(ctx, caseID); err != nil {
		return nil, err
	}
	return s.store.ListEvents(ctx, caseID)
}
func (s *Service) Archive(ctx context.Context, caseID string) (*store.ArchiveRecord, error) {
	return s.store.GetArchive(ctx, caseID)
}
func (s *Service) VerifyArchive(ctx context.Context, caseID string) (*archive.Verification, error) {
	return archive.Verify(ctx, s.store, caseID)
}
func (s *Service) ArchiveVerificationReport(ctx context.Context, caseID string) (*archive.VerificationReport, error) {
	return archive.BuildVerificationReport(ctx, s.store, caseID)
}
func (s *Service) Healthy(ctx context.Context) error { return s.store.Ping(ctx) }
