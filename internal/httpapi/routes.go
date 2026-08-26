package httpapi

func (a *API) registerRoutes() {
	a.mux.HandleFunc("GET /healthz", a.HandleHealth)
	a.mux.HandleFunc("POST /api/v1/cases", a.HandleCreateCase)
	a.mux.HandleFunc("GET /api/v1/cases/{case_id}", a.HandleGetCase)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/baseline/freeze", a.HandleFreezeBaseline)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/baseline/components/revise", a.HandleReviseBaselineComponents)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/risk/assess", a.HandleAssessRisk)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/plan", a.HandleSubmitPlan)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/plan/approve", a.HandleApprovePlan)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/plan/return", a.HandleReturnPlan)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/executions/batch", a.HandleExecutionBatch)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/zones/{zone_id}/executions", a.HandleExecution)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/zones/{zone_id}/remediation", a.HandleRemediation)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/zones/{zone_id}/observations", a.HandleObservation)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/zones/{zone_id}/evaluate", a.HandleEvaluateZone)
	a.mux.HandleFunc("POST /api/v1/cases/{case_id}/review", a.HandleReview)
	a.mux.HandleFunc("GET /api/v1/cases/{case_id}/timeline", a.HandleTimeline)
	a.mux.HandleFunc("GET /api/v1/cases/{case_id}/monitoring/summary", a.HandleMonitoringSummary)
	a.mux.HandleFunc("GET /api/v1/cases/{case_id}/archive", a.HandleArchive)
	a.mux.HandleFunc("GET /api/v1/cases/{case_id}/archive/verify", a.HandleVerifyArchive)
	a.mux.HandleFunc("GET /api/v1/cases/{case_id}/archive/verification-report", a.HandleArchiveVerificationReport)
}
