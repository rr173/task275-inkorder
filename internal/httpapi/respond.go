package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"task275-inkorder/internal/model"
)

// statusByCode 领域错误码 → HTTP 状态码。
var statusByCode = map[string]int{
	model.ErrCodeNotFound:    http.StatusNotFound,
	model.ErrCodeConflict:    http.StatusConflict,
	model.ErrCodeBadState:    http.StatusConflict,
	model.ErrCodeInvalid:     http.StatusBadRequest,
	model.ErrCodeFrozen:      http.StatusConflict,
	model.ErrCodeDuplicate:   http.StatusConflict,
	model.ErrCodeCycle:       http.StatusConflict,
	model.ErrCodeUnsupported: http.StatusBadRequest,
}

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// writeError 把领域错误映射为 HTTP 响应；未知错误返回 500。
// 必须走 errors.As / AsAppError，才能识别被 %w 包装的领域错误。
func writeError(w http.ResponseWriter, err error) {
	ae, _ := err.(*model.AppError)
	if ae != nil {
		status := statusByCode[ae.Code]
		if status == 0 {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]string{"code": ae.Code, "message": ae.Message})
		return
	}
	log.Printf("internal error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{
		"code": "INTERNAL", "message": "internal error",
	})
}

// parseID 解析路径中的正整数 ID。
func parseID(r *http.Request, key string) (int64, error) {
	v := r.PathValue(key)
	var id int64
	if _, err := fmtSscan(v, &id); err != nil || id <= 0 {
		return 0, model.NewError(model.ErrCodeInvalid, "无效的 %s", key)
	}
	return id, nil
}

func fmtSscan(v string, out *int64) (int, error) {
	if v == "" {
		return 0, errors.New("empty")
	}
	var n int64
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int64(c-'0')
	}
	*out = n
	return 1, nil
}
