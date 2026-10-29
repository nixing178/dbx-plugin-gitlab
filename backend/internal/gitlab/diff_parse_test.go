package gitlab

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// GitLab 不同版本对 a_mode / b_mode / generated_file 的类型不一致（数字 vs 字符串），
// 这些字段前端根本不使用，不能因为它们让整个 MR 的 diff 解析失败。
func TestMRDiffsToleratesUnexpectedFieldTypes(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"diff_refs": {"base_sha": "a", "head_sha": "b", "start_sha": "c"},
			"changes": [
				{
					"old_path": "src/a.kt",
					"new_path": "src/a.kt",
					"a_mode": 100644,
					"b_mode": "100644",
					"new_file": false,
					"renamed_file": false,
					"deleted_file": false,
					"too_large": false,
					"collapsed": null,
					"generated_file": "no",
					"diff": "@@ -1 +1 @@\n-a\n+b"
				}
			]
		}`))
	})

	page, err := client.MRDiffs(context.Background(), "demo/app", 9, 1, 5)
	if err != nil {
		t.Fatalf("MRDiffs: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %d", len(page.Items))
	}
	if page.Items[0].NewPath != "src/a.kt" || page.Items[0].Diff != "@@ -1 +1 @@\n-a\n+b" {
		t.Fatalf("file = %+v", page.Items[0])
	}
}

// 解析失败时要把 Go 的具体原因带出来，否则只能看到「格式错误」四个字。
func TestGetJSONLimitedReportsDecodeReason(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"changes":[{"new_path":"a.kt"},]}`))
	})

	var out map[string]any
	_, err := client.getJSONLimited(context.Background(), "/bad-json", nil, &out, 1<<20)
	if err == nil {
		t.Fatal("expected an error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err = %T", err)
	}
	if !strings.Contains(apiErr.Message, "invalid character") {
		t.Fatalf("message = %q, want the underlying decode reason", apiErr.Message)
	}
	if !strings.Contains(apiErr.Message, `starts with: {"changes"`) {
		t.Fatalf("message = %q, want a response snippet", apiErr.Message)
	}
}
