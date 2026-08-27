package archive_verification_cancellation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"timber-pest-remediation-ledger/internal/archive"
	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

type cancellationSource struct {
	caseRecord    *domain.RemediationCase
	archiveRecord *store.ArchiveRecord
}

func (s *cancellationSource) GetCase(ctx context.Context, _ string) (*domain.RemediationCase, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.caseRecord, nil
}

func (s *cancellationSource) ListEvents(ctx context.Context, _ string) ([]domain.AuditEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []domain.AuditEvent{}, nil
}

func (s *cancellationSource) GetArchive(ctx context.Context, _ string) (*store.ArchiveRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.archiveRecord, nil
}

func (s *cancellationSource) ReadNormalizedFacts(ctx context.Context, _ string) (*store.NormalizedFacts, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &store.NormalizedFacts{}, nil
}

func TestArchiveVerificationPropagatesCancellation(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	c := &domain.RemediationCase{
		CaseID:     "cancelled-verification",
		Status:     domain.StatusArchived,
		FrozenAt:   &now,
		CreatedAt:  now,
		Components: []domain.TimberComponent{},
		Review:     &domain.ReviewDecision{Decision: "pass", DecidedAt: now},
	}
	manifest, digest, err := archive.Build(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	c.ArchiveDigest = digest
	source := &cancellationSource{
		caseRecord:    c,
		archiveRecord: &store.ArchiveRecord{CaseID: c.CaseID, Manifest: manifest, Digest: digest, CreatedAt: now},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := archive.Verify(ctx, source, c.CaseID); !errors.Is(err, context.Canceled) {
		t.Errorf("Verify 未传播 context 取消，得到 %v", err)
	}
	if _, err := archive.BuildVerificationReport(ctx, source, c.CaseID); !errors.Is(err, context.Canceled) {
		t.Errorf("BuildVerificationReport 未传播 context 取消，得到 %v", err)
	}
}
