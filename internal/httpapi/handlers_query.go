package httpapi

import (
	"encoding/json"
	"net/http"

	"timber-pest-remediation-ledger/internal/domain"
)

func (a *API) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if err := a.service.Healthy(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "unhealthy", "数据库不可用")
		return
	}
	writeData(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) HandleGetCase(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.Get(r.Context(), caseID)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func (a *API) HandleMonitoringSummary(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	result, err := a.service.MonitoringSummary(r.Context(), caseID)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, result)
}

type timelineEvent struct {
	EventID        string          `json:"event_id"`
	CaseID         string          `json:"case_id"`
	Sequence       int64           `json:"sequence"`
	EventType      string          `json:"event_type"`
	ActorID        string          `json:"actor_id"`
	OccurredAt     string          `json:"occurred_at"`
	Payload        json.RawMessage `json:"payload"`
	PreviousDigest string          `json:"previous_digest"`
	EventDigest    string          `json:"event_digest"`
	RequestID      string          `json:"request_id"`
}

func (a *API) HandleTimeline(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	events, err := a.service.Timeline(r.Context(), caseID)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	output := make([]timelineEvent, len(events))
	for i, event := range events {
		output[i] = eventDTO(event)
	}
	writeData(w, http.StatusOK, output)
}

func eventDTO(event domain.AuditEvent) timelineEvent {
	return timelineEvent{EventID: event.EventID, CaseID: event.CaseID, Sequence: event.Sequence, EventType: event.EventType, ActorID: event.ActorID, OccurredAt: event.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), Payload: json.RawMessage(event.Payload), PreviousDigest: event.PreviousDigest, EventDigest: event.EventDigest, RequestID: event.RequestID}
}

func (a *API) HandleArchive(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	record, err := a.service.Archive(r.Context(), caseID)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"case_id": record.CaseID, "terminal_revision": record.TerminalRevision, "digest": record.Digest, "manifest": json.RawMessage(record.Manifest), "created_at": record.CreatedAt})
}

func (a *API) HandleVerifyArchive(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	verification, err := a.service.VerifyArchive(r.Context(), caseID)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	status := http.StatusOK
	if !verification.Valid {
		status = http.StatusConflict
	}
	writeData(w, status, verification)
}

func (a *API) HandleArchiveVerificationReport(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	report, err := a.service.ArchiveVerificationReport(r.Context(), caseID)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	status := http.StatusOK
	if !report.Valid {
		status = http.StatusConflict
	}
	writeData(w, status, report)
}
