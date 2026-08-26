package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"timber-pest-remediation-ledger/internal/cases"
	"timber-pest-remediation-ledger/internal/domain"
)

const maxRequestBody = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return domain.Invalid("content_type_invalid", "Content-Type 必须为 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return domain.Invalid("body_too_large", "请求体超过 %d 字节", maxRequestBody)
		}
		return domain.Invalid("json_invalid", "JSON 请求无效: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.Invalid("json_trailing_data", "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func checkMetaHeaders(r *http.Request, meta cases.CommandMeta) error {
	if value := strings.TrimSpace(r.Header.Get("X-Request-ID")); value != "" && value != meta.RequestID {
		return domain.Invalid("request_id_mismatch", "X-Request-ID 与 request_id 不一致")
	}
	if value := strings.TrimSpace(r.Header.Get("X-Actor-ID")); value != "" && value != meta.ActorID {
		return domain.Invalid("actor_id_mismatch", "X-Actor-ID 与 actor_id 不一致")
	}
	if value := strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), `\"`); value != "" {
		revision, err := strconv.ParseInt(value, 10, 64)
		if err != nil || revision != meta.ExpectedRevision {
			return domain.Invalid("revision_header_mismatch", "If-Match 与 expected_revision 不一致")
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": data})
}

func writeMappedError(w http.ResponseWriter, err error) {
	var business *domain.BusinessError
	if errors.As(err, &business) {
		status := http.StatusBadRequest
		switch business.Kind {
		case domain.KindNotFound:
			status = http.StatusNotFound
		case domain.KindConflict:
			status = http.StatusConflict
		case domain.KindGate:
			status = http.StatusUnprocessableEntity
		case domain.KindCorrupt:
			status = http.StatusInternalServerError
		}
		writeError(w, status, business.Code, business.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", "服务内部错误")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func pathID(r *http.Request, name string) (string, error) {
	value := strings.TrimSpace(r.PathValue(name))
	if value == "" || len(value) > 128 || strings.ContainsAny(value, "/\\") {
		return "", domain.Invalid("path_id_invalid", "%s 路径参数无效", name)
	}
	return value, nil
}

func requireMeta(w http.ResponseWriter, r *http.Request, target any, meta cases.CommandMeta) bool {
	if err := checkMetaHeaders(r, meta); err != nil {
		writeMappedError(w, err)
		return false
	}
	return true
}

func requestLabel(r *http.Request) string { return fmt.Sprintf("%s %s", r.Method, r.URL.Path) }
