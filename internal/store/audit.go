package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"timber-pest-remediation-ledger/internal/domain"
)

func buildAuditEvent(caseID string, sequence int64, previous, requestID string, draft domain.EventDraft, occurredAt time.Time) (domain.AuditEvent, error) {
	payload, err := json.Marshal(draft.Payload)
	if err != nil {
		return domain.AuditEvent{}, fmt.Errorf("编码审计载荷: %w", err)
	}
	timestamp := occurredAt.UTC().Format(time.RFC3339Nano)
	h := sha256.New()
	for _, part := range []string{previous, caseID, strconv.FormatInt(sequence, 10), draft.Type, draft.ActorID, timestamp, string(payload), requestID} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	digest := hex.EncodeToString(h.Sum(nil))
	return domain.AuditEvent{EventID: digest[:24], CaseID: caseID, Sequence: sequence, EventType: draft.Type, ActorID: draft.ActorID, OccurredAt: occurredAt.UTC(), Payload: payload, PreviousDigest: previous, EventDigest: digest, RequestID: requestID}, nil
}

func verifyEvent(previous string, event domain.AuditEvent) error {
	draft := domain.EventDraft{Type: event.EventType, ActorID: event.ActorID, Payload: json.RawMessage(event.Payload)}
	rebuilt, err := buildAuditEvent(event.CaseID, event.Sequence, previous, event.RequestID, draft, event.OccurredAt)
	if err != nil {
		return err
	}
	// RawMessage is embedded verbatim by json.Marshal, preserving canonical persisted payload bytes.
	if rebuilt.EventDigest != event.EventDigest {
		return fmt.Errorf("案件 %s 的事件 %d 摘要不匹配", event.CaseID, event.Sequence)
	}
	if event.PreviousDigest != previous {
		return fmt.Errorf("案件 %s 的事件 %d 前序摘要不匹配", event.CaseID, event.Sequence)
	}
	return nil
}
