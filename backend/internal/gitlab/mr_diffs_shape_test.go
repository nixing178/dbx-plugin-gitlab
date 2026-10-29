package gitlab

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// GitLab 的 /merge_requests/:iid/diffs 返回的是「文件数组」，
// 不带 diff_refs 包壳；老代码按 {"changes": [...]} 解，永远解不出来。
func TestMRDiffsAcceptsArrayResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total", "2")
		_, _ = w.Write([]byte(`[
			{"old_path":"a.kt","new_path":"a.kt","new_file":false,"renamed_file":false,"deleted_file":false,"too_large":false,"diff":"@@ -1 +1 @@"},
			{"old_path":"b.kt","new_path":"b.kt","new_file":true,"renamed_file":false,"deleted_file":false,"too_large":false,"diff":"@@ -0,0 +1 @@"}
		]`))
	})

	page, err := client.MRDiffs(context.Background(), "demo/app", 9, 1, 5)
	if err != nil {
		t.Fatalf("MRDiffs: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %d", len(page.Items))
	}
	if page.Items[1].NewPath != "b.kt" || !page.Items[1].NewFile {
		t.Fatalf("second = %+v", page.Items[1])
	}
}

// 仍要兼容带包壳的实例（部分版本/changes 端点会给 {changes, diff_refs}）。
func TestMRDiffsAcceptsEnvelopeResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"diff_refs": {"base_sha": "a", "head_sha": "b", "start_sha": "c"},
			"changes": [
				{"old_path":"a.kt","new_path":"a.kt","new_file":false,"renamed_file":false,"deleted_file":false,"too_large":false,"diff":"@@ -1 +1 @@"}
			]
		}`))
	})

	page, err := client.MRDiffs(context.Background(), "demo/app", 9, 1, 5)
	if err != nil {
		t.Fatalf("MRDiffs: %v", err)
	}
	if len(page.Items) != 1 || page.DiffRefs.HeadSHA != "b" {
		t.Fatalf("page = %+v", page)
	}
}

// 解析失败时提示里必须是「响应开头」，不能被 bufio 的缓冲复用覆盖成中段内容。
func TestGetJSONLimitedSnippetKeepsResponseHead(t *testing.T) {
	filler := strings.Repeat("x", 4096)
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"changes":[{"diff":"` + filler + `"},]}`))
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
	if !strings.Contains(apiErr.Message, `starts with: {"changes"`) {
		t.Fatalf("message = %q, want the real response head", apiErr.Message)
	}
}
