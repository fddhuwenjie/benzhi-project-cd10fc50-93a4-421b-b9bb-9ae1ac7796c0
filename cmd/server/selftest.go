package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/httpapi"
	"timber-pest-remediation-ledger/internal/store"
)

type selfTestEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
type selfTestCase struct {
	CaseID        string `json:"case_id"`
	Status        string `json:"status"`
	Revision      int64  `json:"revision"`
	ArchiveDigest string `json:"archive_digest"`
}

func runSelfTest(ctx context.Context, address string) error {
	directory, err := os.MkdirTemp("", "timber-ledger-selftest-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	repository, err := store.Open(ctx, filepath.Join(directory, "selftest.db"))
	if err != nil {
		return err
	}
	defer repository.Close()
	service := cases.NewService(repository)
	defer service.Close()
	server := httpapi.NewHTTPServer(address, httpapi.New(service).Handler())
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("自检监听 %s: %w", address, err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
		<-serveErr
	}()
	client := &http.Client{Timeout: 3 * time.Second}
	baseURL := "http://" + address
	if err := waitHealthy(ctx, client, baseURL+"/healthz"); err != nil {
		return err
	}
	caseID := "selftest-case"
	current, err := selfTestPost(ctx, client, baseURL+"/api/v1/cases", map[string]any{"request_id": "st-01", "actor_id": "survey-1", "case_id": caseID, "site_name": "自检文物建筑", "building_zone": "正殿东次间", "survey_lead_id": "survey-1", "components": []map[string]any{{"component_id": "beam-1", "location_code": "ZD-E-01", "component_type": "梁", "heritage_grade": "I", "pest_clue": "蛀屑与羽化孔", "activity_score": 4, "damage_extent_percent": 28.5, "moisture_percent": 16.2, "evidence_digest": "sha256:survey-evidence"}}})
	if err != nil {
		return err
	}
	commands := []struct {
		suffix string
		body   func(int64) map[string]any
	}{
		{"/baseline/freeze", metaBody("st-02", "survey-1")},
		{"/risk/assess", metaBody("st-03", "engineer-1")},
		{"/plan", func(revision int64) map[string]any {
			body := metaBody("st-04", "planner-1")(revision)
			body["zones"] = []map[string]any{{"zone_id": "zone-east", "component_ids": []string{"beam-1"}, "method": "低压注射药剂", "approved_parameters": map[string]float64{"concentration_percent": 2.5, "dose_ml": 120}, "protection_constraints": []string{"隔离彩画层", "收集残液"}, "responsible_id": "field-1", "acceptance_thresholds": map[string]any{"max_activity_count": 0, "max_acoustic_score": 0.2, "allow_visual_activity": false, "min_observations": 1, "observation_window_hours": 0}}}
			return body
		}},
		{"/plan/approve", metaBody("st-05", "approver-1")},
		{"/zones/zone-east/executions", func(revision int64) map[string]any {
			body := metaBody("st-06", "field-1")(revision)
			body["execution"] = map[string]any{"actual_parameters": map[string]float64{"concentration_percent": 2.5, "dose_ml": 120}, "evidence_digest": "sha256:execution-evidence", "deviation_note": "无偏差"}
			return body
		}},
		{"/zones/zone-east/observations", func(revision int64) map[string]any {
			body := metaBody("st-07", "monitor-1")(revision)
			body["observation"] = map[string]any{"observation_id": "observation-1", "method": "trap", "activity_count": 0, "acoustic_score": 0.1, "visual_finding": "none", "evidence_digest": "sha256:monitor-evidence"}
			return body
		}},
		{"/zones/zone-east/evaluate", metaBody("st-08", "engineer-1")},
		{"/review", func(revision int64) map[string]any {
			body := metaBody("st-09", "reviewer-1")(revision)
			body["decision"] = "pass"
			body["findings"] = "职责分离、分区闭合及证据链均符合要求"
			body["evidence_complete"] = true
			return body
		}},
	}
	for _, command := range commands {
		current, err = selfTestPost(ctx, client, baseURL+"/api/v1/cases/"+caseID+command.suffix, command.body(current.Revision))
		if err != nil {
			return err
		}
	}
	if current.Status != "archived" || current.Revision != 9 || current.ArchiveDigest == "" {
		return fmt.Errorf("自检终局不符合预期: status=%s revision=%d", current.Status, current.Revision)
	}
	var verification struct {
		Valid         bool   `json:"valid"`
		RebuiltDigest string `json:"rebuilt_digest"`
	}
	if err := selfTestGet(ctx, client, baseURL+"/api/v1/cases/"+caseID+"/archive/verify", &verification); err != nil {
		return err
	}
	if !verification.Valid || verification.RebuiltDigest != current.ArchiveDigest {
		return fmt.Errorf("归档验证未通过")
	}
	var timeline []json.RawMessage
	if err := selfTestGet(ctx, client, baseURL+"/api/v1/cases/"+caseID+"/timeline", &timeline); err != nil {
		return err
	}
	if len(timeline) != 9 {
		return fmt.Errorf("时间线事件数为 %d，预期 9", len(timeline))
	}
	fmt.Printf("自检通过：案件 %s 已归档，revision=%d，digest=%s\n", caseID, current.Revision, current.ArchiveDigest)
	return nil
}

func metaBody(requestID, actorID string) func(int64) map[string]any {
	return func(revision int64) map[string]any {
		return map[string]any{"request_id": requestID, "actor_id": actorID, "expected_revision": revision}
	}
}

func selfTestPost(ctx context.Context, client *http.Client, url string, body any) (selfTestCase, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return selfTestCase{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return selfTestCase{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return selfTestCase{}, err
	}
	defer response.Body.Close()
	envelope, err := decodeSelfTestResponse(response)
	if err != nil {
		return selfTestCase{}, err
	}
	var c selfTestCase
	if err := json.Unmarshal(envelope.Data, &c); err != nil {
		return c, err
	}
	return c, nil
}

func selfTestGet(ctx context.Context, client *http.Client, url string, output any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	envelope, err := decodeSelfTestResponse(response)
	if err != nil {
		return err
	}
	return json.Unmarshal(envelope.Data, output)
}

func decodeSelfTestResponse(response *http.Response) (selfTestEnvelope, error) {
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return selfTestEnvelope{}, err
	}
	var envelope selfTestEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return envelope, fmt.Errorf("自检响应不是 JSON: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if envelope.Error != nil {
			return envelope, fmt.Errorf("自检 HTTP %d %s: %s", response.StatusCode, envelope.Error.Code, envelope.Error.Message)
		}
		return envelope, fmt.Errorf("自检 HTTP %d", response.StatusCode)
	}
	return envelope, nil
}

func waitHealthy(ctx context.Context, client *http.Client, url string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待健康检查: %w", ctx.Err())
		case <-ticker.C:
			request, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			response, err := client.Do(request)
			if err == nil {
				response.Body.Close()
				if response.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}
