package gitlab

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

// 声明了 Content-Length 但提前断流：这是「超时/连接被掐断」，不能报成格式错误。
func TestGetJSONLimitedMapsTruncatedBodyToTimeout(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "1024")
		_, _ = w.Write([]byte(`{"changes":[{"new_path":"a"`))
	})

	var out map[string]any
	_, err := client.getJSONLimited(context.Background(), "/truncated", nil, &out, 1<<20)
	if err == nil {
		t.Fatal("expected an error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Message != "request timed out" {
		t.Fatalf("message = %q, want request timed out", apiErr.Message)
	}
}

// 普通请求沿用连接配置的超时；diff 走单独放宽的副本，
// 否则大 MR 会在 15s 被硬掐断（表现为 unexpected EOF）。
func TestDiffClientUsesLongerTimeout(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})

	if client.http.Timeout != 15*time.Second {
		t.Fatalf("http.Client.Timeout = %v, want the 15s default", client.http.Timeout)
	}

	diff := client.diffClient()
	if diff.Timeout != maxDiffTimeout {
		t.Fatalf("diff timeout = %v, want %v", diff.Timeout, maxDiffTimeout)
	}
	if diff.Transport != client.http.Transport {
		t.Fatal("diff client must reuse the shared transport")
	}
}
