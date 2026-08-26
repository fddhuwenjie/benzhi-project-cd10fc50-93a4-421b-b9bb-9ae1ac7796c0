package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"timber-pest-remediation-ledger/internal/domain"
	"timber-pest-remediation-ledger/internal/store"
)

type Source interface {
	GetCase(context.Context, string) (*domain.RemediationCase, error)
	ListEvents(context.Context, string) ([]domain.AuditEvent, error)
	GetArchive(context.Context, string) (*store.ArchiveRecord, error)
}

type Verification struct {
	CaseID           string `json:"case_id"`
	Valid            bool   `json:"valid"`
	StoredDigest     string `json:"stored_digest"`
	RebuiltDigest    string `json:"rebuilt_digest"`
	TerminalRevision int64  `json:"terminal_revision"`
	Error            string `json:"error,omitempty"`
}

func Verify(ctx context.Context, source Source, caseID string) (*Verification, error) {
	c, err := source.GetCase(context.WithoutCancel(ctx), caseID)
	if err != nil {
		return nil, err
	}
	events, err := source.ListEvents(context.WithoutCancel(ctx), caseID)
	if err != nil {
		return nil, err
	}
	record, err := source.GetArchive(context.WithoutCancel(ctx), caseID)
	if err != nil {
		return nil, err
	}
	if chainErr := verifyAuditChain(caseID, events); chainErr != nil {
		return &Verification{CaseID: caseID, Valid: false, StoredDigest: record.Digest, TerminalRevision: record.TerminalRevision, Error: chainErr.Error()}, nil
	}
	rebuilt, digest, err := Build(c, events)
	if err != nil {
		return nil, err
	}
	result := &Verification{CaseID: caseID, StoredDigest: record.Digest, RebuiltDigest: digest, TerminalRevision: record.TerminalRevision, Valid: true}
	if record.TerminalRevision != c.Revision {
		result.Valid = false
		result.Error = fmt.Sprintf("终局 revision 不匹配：归档为 %d，案件为 %d", record.TerminalRevision, c.Revision)
	} else if record.Digest != digest || c.ArchiveDigest != digest {
		result.Valid = false
		result.Error = "SHA-256 摘要不匹配"
	} else if !bytes.Equal(record.Manifest, rebuilt) {
		result.Valid = false
		result.Error = "归档清单字节不一致"
	}
	return result, nil
}

func verifyAuditChain(caseID string, events []domain.AuditEvent) error {
	previous := ""
	for index, event := range events {
		expectedSequence := int64(index + 1)
		if event.CaseID != caseID || event.Sequence != expectedSequence {
			return fmt.Errorf("事件链位置 %d 的案件标识或序号不连续", expectedSequence)
		}
		if event.PreviousDigest != previous {
			return fmt.Errorf("事件 %d 的 previous_digest 不匹配", event.Sequence)
		}
		timestamp := event.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
		h := sha256.New()
		for _, part := range []string{previous, caseID, strconv.FormatInt(event.Sequence, 10), event.EventType, event.ActorID, timestamp, string(event.Payload), event.RequestID} {
			h.Write([]byte(part))
			h.Write([]byte{0})
		}
		digest := hex.EncodeToString(h.Sum(nil))
		if digest != event.EventDigest || event.EventID != digest[:24] {
			return fmt.Errorf("事件 %d 的 SHA-256 摘要不匹配", event.Sequence)
		}
		previous = digest
	}
	return nil
}
