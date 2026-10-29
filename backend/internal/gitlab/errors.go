package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// 错误种类。Message 统一以 "<kind>: <detail>" 形式返回，UI 依 kind 做本地化。
const (
	ErrInvalidConfig = "invalid_config"
	ErrUnauthorized  = "unauthorized"
	ErrForbidden     = "forbidden"
	ErrNotFound      = "not_found"
	ErrConflict      = "conflict"
	ErrRateLimited   = "rate_limited"
	ErrServer        = "server_error"
	ErrNetwork       = "network"
	ErrReadOnly      = "read_only"
)

// APIError 是 GitLab 客户端与 RPC 层共用的语义化错误。
type APIError struct {
	Kind       string
	Status     int
	Message    string
	RetryAfter int
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return e.Kind
	}
	return e.Kind + ": " + e.Message
}

// mapHTTPError 把 HTTP 响应转换为语义化错误。
// GitLab 的错误体通常是 {"message":"..."}，部分场景是数组或对象。
func mapHTTPError(resp *http.Response, body []byte) *APIError {
	kind := ErrServer
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		kind = ErrUnauthorized
	case http.StatusForbidden:
		kind = ErrForbidden
	case http.StatusNotFound:
		kind = ErrNotFound
	case http.StatusConflict:
		kind = ErrConflict
	case http.StatusTooManyRequests:
		kind = ErrRateLimited
	}

	message := extractMessage(body)
	if message == "" {
		message = strings.ToLower(http.StatusText(resp.StatusCode))
	}

	err := &APIError{Kind: kind, Status: resp.StatusCode, Message: message}
	if kind == ErrRateLimited {
		if seconds, convErr := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After"))); convErr == nil {
			err.RetryAfter = seconds
		}
	}
	return err
}

// extractMessage 兼容 GitLab 的多种错误体：
// {"message":"404 Not Found"} / {"message":["label is invalid"]} / {"error":"invalid_grant"}。
func extractMessage(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return truncate(trimmed, 300)
	}

	for _, key := range []string{"message", "error_description", "error"} {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		case []any:
			parts := make([]string, 0, len(typed))
			for _, item := range typed {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, text)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "; ")
			}
		case map[string]any:
			if encoded, err := json.Marshal(typed); err == nil {
				return truncate(string(encoded), 300)
			}
		}
	}
	return ""
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func invalidConfig(format string, args ...any) *APIError {
	return &APIError{Kind: ErrInvalidConfig, Message: fmt.Sprintf(format, args...)}
}
