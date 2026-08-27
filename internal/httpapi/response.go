// Package httpapi 提供 /api 前缀的 HTTP 接口，统一 JSON 信封与错误映射。
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"task288-dropletlineage/internal/model"
)

// envelope 是统一响应信封：成功带 data，失败带 error。
type envelope struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// writeJSON 输出 JSON 响应。
func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(envelope{Data: data}); err != nil {
		log.Printf("write json: %v", err)
	}
}

// writeErr 输出错误信封，并把领域错误映射为 HTTP 状态码。
func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, model.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, model.ErrConflict),
		errors.Is(err, model.ErrStateMachine),
		errors.Is(err, model.ErrEventClosed),
		errors.Is(err, model.ErrDropletIDDuplicate):
		code = http.StatusConflict
	case errors.Is(err, model.ErrInvalid),
		errors.Is(err, model.ErrSequenceRegression),
		errors.Is(err, model.ErrCoordOutOfRange),
		errors.Is(err, model.ErrNoFrozenVersion),
		errors.Is(err, model.ErrFrozen):
		code = http.StatusUnprocessableEntity
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(envelope{Error: err.Error()})
}

// decode 解析请求体 JSON 到目标结构。
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
