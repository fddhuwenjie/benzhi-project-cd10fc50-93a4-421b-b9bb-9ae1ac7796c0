package httpapi

import (
	"context"
	"net/http"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/domain"
)

func (a *API) HandleCreateCase(w http.ResponseWriter, r *http.Request) {
	var command cases.CreateCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.Create(r.Context(), command)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusCreated, c)
}

func (a *API) HandleFreezeBaseline(w http.ResponseWriter, r *http.Request) {
	a.handleMetaCommand(w, r, a.service.FreezeBaseline)
}

func (a *API) HandleReviseBaselineComponents(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	var command cases.ComponentRevisionCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeMappedError(w, err)
		return
	}
	if err := checkMetaHeaders(r, command.CommandMeta); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.ReviseBaselineComponents(r.Context(), caseID, command)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func (a *API) HandleAssessRisk(w http.ResponseWriter, r *http.Request) {
	a.handleMetaCommand(w, r, a.service.AssessRisk)
}

func (a *API) HandleApprovePlan(w http.ResponseWriter, r *http.Request) {
	a.handleMetaCommand(w, r, a.service.ApprovePlan)
}

func (a *API) HandleReturnPlan(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	var command cases.PlanReturnCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeMappedError(w, err)
		return
	}
	if err := checkMetaHeaders(r, command.CommandMeta); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.ReturnPlan(r.Context(), caseID, command)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func (a *API) HandleSubmitPlan(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	var command cases.PlanCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeMappedError(w, err)
		return
	}
	if err := checkMetaHeaders(r, command.CommandMeta); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.SubmitPlan(r.Context(), caseID, command)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func (a *API) HandleExecution(w http.ResponseWriter, r *http.Request) {
	caseID, zoneID, ok := commandPath(w, r)
	if !ok {
		return
	}
	var command cases.ExecutionCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeMappedError(w, err)
		return
	}
	if err := checkMetaHeaders(r, command.CommandMeta); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.RecordExecution(r.Context(), caseID, zoneID, command)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func (a *API) HandleExecutionBatch(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	var command cases.ExecutionBatchCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeMappedError(w, err)
		return
	}
	if err := checkMetaHeaders(r, command.CommandMeta); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.RecordExecutionBatch(r.Context(), caseID, command)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func (a *API) HandleRemediation(w http.ResponseWriter, r *http.Request) {
	caseID, zoneID, ok := commandPath(w, r)
	if !ok {
		return
	}
	var command cases.RemediationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeMappedError(w, err)
		return
	}
	if err := checkMetaHeaders(r, command.CommandMeta); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.DecideRemediation(r.Context(), caseID, zoneID, command)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func (a *API) HandleObservation(w http.ResponseWriter, r *http.Request) {
	caseID, zoneID, ok := commandPath(w, r)
	if !ok {
		return
	}
	var command cases.ObservationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeMappedError(w, err)
		return
	}
	if err := checkMetaHeaders(r, command.CommandMeta); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.AddObservation(r.Context(), caseID, zoneID, command)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func (a *API) HandleEvaluateZone(w http.ResponseWriter, r *http.Request) {
	caseID, zoneID, ok := commandPath(w, r)
	if !ok {
		return
	}
	var meta cases.CommandMeta
	if err := decodeJSON(w, r, &meta); err != nil {
		writeMappedError(w, err)
		return
	}
	if err := checkMetaHeaders(r, meta); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.EvaluateZone(r.Context(), caseID, zoneID, meta)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func (a *API) HandleReview(w http.ResponseWriter, r *http.Request) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	var command cases.ReviewCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeMappedError(w, err)
		return
	}
	if err := checkMetaHeaders(r, command.CommandMeta); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := a.service.Review(r.Context(), caseID, command)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func (a *API) handleMetaCommand(w http.ResponseWriter, r *http.Request, action func(context.Context, string, cases.CommandMeta) (*domain.RemediationCase, error)) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return
	}
	var meta cases.CommandMeta
	if err := decodeJSON(w, r, &meta); err != nil {
		writeMappedError(w, err)
		return
	}
	if err := checkMetaHeaders(r, meta); err != nil {
		writeMappedError(w, err)
		return
	}
	c, err := action(r.Context(), caseID, meta)
	if err != nil {
		writeMappedError(w, err)
		return
	}
	writeData(w, http.StatusOK, c)
}

func commandPath(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	caseID, err := pathID(r, "case_id")
	if err != nil {
		writeMappedError(w, err)
		return "", "", false
	}
	zoneID, err := pathID(r, "zone_id")
	if err != nil {
		writeMappedError(w, err)
		return "", "", false
	}
	return caseID, zoneID, true
}
