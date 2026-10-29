package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestCompareSendsRefsAndDropsDiffBody(t *testing.T) {
	var escaped string
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query()
		_, _ = w.Write([]byte(`{
			"commits":[
				{"id":"dcCBFbEf","short_id":"dcCBFbEf","title":"refactor(sample): 移除示例接口"},
				{"id":"7dF63a57","short_id":"7dF63a57","title":"refactor(sample): 调整示例服务调用"}
			],
			"diffs":[
				{"old_path":"a.kt","new_path":"a.kt","diff":"@@ -1 +1,2 @@\n-old\n+new\n+more"},
				{"old_path":"b.kt","new_path":"b.kt","new_file":true,"diff":"@@ -0,0 +1 @@\n+hello"}
			],
			"compare_timeout":false,
			"compare_same_ref":false
		}`))
	})

	result, err := client.Compare(
		context.Background(),
		"demo/project",
		"master",
		"feature/sample-change",
	)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fproject/repository/compare" {
		t.Fatalf("path = %q", escaped)
	}
	if query.Get("from") != "master" || query.Get("to") != "feature/sample-change" {
		t.Fatalf("query = %v", query)
	}
	if query.Get("straight") != "true" {
		t.Fatalf("straight = %q", query.Get("straight"))
	}
	if len(result.Commits) != 2 {
		t.Fatalf("commits = %+v", result.Commits)
	}
	if result.Commits[1].Title != "refactor(sample): 调整示例服务调用" {
		t.Fatalf("commit title = %q", result.Commits[1].Title)
	}
	if len(result.Diffs) != 2 {
		t.Fatalf("diffs = %+v", result.Diffs)
	}
	if result.Diffs[0].Additions != 2 || result.Diffs[0].Deletions != 1 {
		t.Fatalf("diff stats = %+v", result.Diffs[0])
	}
	if result.Diffs[0].Diff != "" {
		t.Fatalf("diff body must be dropped: %q", result.Diffs[0].Diff)
	}
	if !result.Diffs[1].NewFile {
		t.Fatalf("new file = %+v", result.Diffs[1])
	}
}

func TestCompareRequiresBothRefs(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
	})
	if _, err := client.Compare(context.Background(), "demo/project", "", "master"); err == nil {
		t.Fatal("expected an error for a missing ref")
	}
	if _, err := client.Compare(context.Background(), "demo/project", "master", " "); err == nil {
		t.Fatal("expected an error for a blank ref")
	}
}

func TestCompareReportsTimeout(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"commits":[],"diffs":[],"compare_timeout":true,"compare_same_ref":false}`))
	})

	result, err := client.Compare(context.Background(), "demo/project", "master", "feature/x")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !result.CompareTimeout {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Commits) != 0 || len(result.Diffs) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
