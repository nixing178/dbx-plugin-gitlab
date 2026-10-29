package gitlab

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// 响应超过上限时必须报「response too large」，不能伪装成 JSON 格式错误。
func TestGetJSONLimitedReportsOversize(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":["` + strings.Repeat("x", 2048) + `"]}`))
	})

	var out map[string]any
	_, err := client.getJSONLimited(context.Background(), "/oversized", nil, &out, 128)
	if err == nil {
		t.Fatal("expected an error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Message != "response too large" {
		t.Fatalf("message = %q", apiErr.Message)
	}
}

// 上限内的响应要正常解码。
func TestGetJSONLimitedDecodesWithinLimit(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"value":42}`))
	})

	var out map[string]any
	if _, err := client.getJSONLimited(context.Background(), "/ok", nil, &out, 1024); err != nil {
		t.Fatalf("getJSONLimited: %v", err)
	}
	if out["value"] != float64(42) {
		t.Fatalf("out = %v", out)
	}
}

// MR diff 走单独的更大上限：36 个文件的重构 MR 会超过默认的 8MB。
func TestMRDiffsUsesLargeLimit(t *testing.T) {
	big := strings.Repeat("x", 9<<20)
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"diff_refs":{"head_sha":"abc"},"changes":[{"new_path":"a.go","diff":"` + big + `"}]}`,
		))
	})

	page, err := client.MRDiffs(context.Background(), "demo/app", 9, 1, 20)
	if err != nil {
		t.Fatalf("MRDiffs: %v", err)
	}
	if len(page.Items) != 1 || len(page.Items[0].Diff) != 9<<20 {
		t.Fatalf("items = %d, diff len = %d", len(page.Items), len(page.Items[0].Diff))
	}
}
