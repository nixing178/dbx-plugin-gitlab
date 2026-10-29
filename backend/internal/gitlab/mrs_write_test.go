package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

// newWriteClient 捕获写请求的方法、路径与 JSON 请求体。
func newWriteClient(
	t *testing.T,
	handler func(w http.ResponseWriter, r *http.Request, body map[string]any),
) *Client {
	t.Helper()
	return newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&payload)
		}
		handler(w, r, payload)
	})
}

func TestCommentMRPostsDiscussion(t *testing.T) {
	var method, escaped string
	var payload map[string]any

	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		method = r.Method
		escaped = r.URL.EscapedPath()
		payload = body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"d1","notes":[{"id":31,"body":"LGTM"}]}`))
	})

	note, err := client.CommentMR(context.Background(), "demo/app", 9, "LGTM", nil)
	if err != nil {
		t.Fatalf("CommentMR: %v", err)
	}
	if method != http.MethodPost {
		t.Fatalf("method = %s", method)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/merge_requests/9/discussions" {
		t.Fatalf("path = %q", escaped)
	}
	if payload["body"] != "LGTM" {
		t.Fatalf("body = %v", payload["body"])
	}
	if note == nil || note.ID != 31 || note.Body != "LGTM" {
		t.Fatalf("note = %+v", note)
	}
}

func TestCommentMRWithInlinePosition(t *testing.T) {
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, _ *http.Request, body map[string]any) {
		payload = body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"d2","notes":[{"id":32,"body":"这里要判空"}]}`))
	})

	position := &PositionInput{
		BaseSHA:      "base1",
		HeadSHA:      "head2",
		StartSHA:     "start3",
		NewPath:      "a.go",
		OldPath:      "a.go",
		PositionType: "text",
		NewLine:      12,
	}
	if _, err := client.CommentMR(context.Background(), "demo/app", 9, "这里要判空", position); err != nil {
		t.Fatalf("CommentMR: %v", err)
	}

	raw, ok := payload["position"].(map[string]any)
	if !ok {
		t.Fatalf("position missing: %v", payload)
	}
	if raw["base_sha"] != "base1" || raw["head_sha"] != "head2" || raw["new_path"] != "a.go" {
		t.Fatalf("position = %v", raw)
	}
	if raw["new_line"] != float64(12) {
		t.Fatalf("new_line = %v", raw["new_line"])
	}
}

func TestReplyDiscussion(t *testing.T) {
	var escaped string
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		escaped = r.URL.EscapedPath()
		payload = body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":33,"body":"已修复"}`))
	})

	note, err := client.ReplyDiscussion(context.Background(), "demo/app", 9, "disc-1", "已修复")
	if err != nil {
		t.Fatalf("ReplyDiscussion: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/merge_requests/9/discussions/disc-1/notes" {
		t.Fatalf("path = %q", escaped)
	}
	if payload["body"] != "已修复" {
		t.Fatalf("body = %v", payload["body"])
	}
	if note == nil || note.ID != 33 {
		t.Fatalf("note = %+v", note)
	}
}

func TestResolveDiscussion(t *testing.T) {
	var method string
	var query url.Values
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		method = r.Method
		query = r.URL.Query()
		_, _ = w.Write([]byte(`{"id":"disc-1","notes":[{"id":33,"resolved":true}]}`))
	})

	discussion, err := client.ResolveDiscussion(context.Background(), "demo/app", 9, "disc-1", true)
	if err != nil {
		t.Fatalf("ResolveDiscussion: %v", err)
	}
	if method != http.MethodPut {
		t.Fatalf("method = %s", method)
	}
	if query.Get("resolved") != "true" {
		t.Fatalf("resolved = %q", query.Get("resolved"))
	}
	if discussion == nil || discussion.ID != "disc-1" {
		t.Fatalf("discussion = %+v", discussion)
	}
}

func TestApproveAndUnapprove(t *testing.T) {
	var calls []string
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		calls = append(calls, r.Method+" "+r.URL.EscapedPath())
		if r.URL.Path == "/api/v4/projects/demo/app/merge_requests/9/approve" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":100,"iid":9,"project_id":12,"state":"opened"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	approval, err := client.ApproveMR(context.Background(), "demo/app", 9, "deadbeef")
	if err != nil {
		t.Fatalf("ApproveMR: %v", err)
	}
	if approval == nil || approval.IID != 9 {
		t.Fatalf("approval = %+v", approval)
	}
	if err := client.UnapproveMR(context.Background(), "demo/app", 9); err != nil {
		t.Fatalf("UnapproveMR: %v", err)
	}

	expected := []string{
		"POST /api/v4/projects/demo%2Fapp/merge_requests/9/approve",
		"POST /api/v4/projects/demo%2Fapp/merge_requests/9/unapprove",
	}
	for index, want := range expected {
		if index >= len(calls) || calls[index] != want {
			t.Fatalf("calls = %v, want %v", calls, expected)
		}
	}
}

func TestMergeMRBody(t *testing.T) {
	var method string
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		method = r.Method
		payload = body
		_, _ = w.Write([]byte(`{"id":100,"iid":9,"state":"merged"}`))
	})

	mr, err := client.MergeMR(context.Background(), "demo/app", 9, MergeOptions{
		SHA:                "deadbeef",
		Squash:             true,
		RemoveSourceBranch: true,
		MergeCommitMessage: "feat: 示例功能",
	})
	if err != nil {
		t.Fatalf("MergeMR: %v", err)
	}
	if method != http.MethodPut {
		t.Fatalf("method = %s", method)
	}
	if payload["sha"] != "deadbeef" || payload["squash"] != true ||
		payload["should_remove_source_branch"] != true ||
		payload["merge_commit_message"] != "feat: 示例功能" {
		t.Fatalf("payload = %v", payload)
	}
	if mr == nil || mr.State != "merged" {
		t.Fatalf("mr = %+v", mr)
	}
}

func TestSetMRState(t *testing.T) {
	var method string
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		method = r.Method
		payload = body
		_, _ = w.Write([]byte(`{"id":100,"iid":9,"state":"closed"}`))
	})

	mr, err := client.SetMRState(context.Background(), "demo/app", 9, "close")
	if err != nil {
		t.Fatalf("SetMRState: %v", err)
	}
	if method != http.MethodPut {
		t.Fatalf("method = %s", method)
	}
	if payload["state_event"] != "close" {
		t.Fatalf("payload = %v", payload)
	}
	if mr == nil || mr.State != "closed" {
		t.Fatalf("mr = %+v", mr)
	}
}

func TestCreateMRBody(t *testing.T) {
	var escaped string
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		escaped = r.URL.EscapedPath()
		payload = body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":101,"iid":10,"title":"feat: 示例功能"}`))
	})

	mr, err := client.CreateMR(context.Background(), "demo/app", CreateMRInput{
		SourceBranch:       "feature/x",
		TargetBranch:       "main",
		Title:              "feat: 示例功能",
		Description:        "描述",
		RemoveSourceBranch: true,
		Squash:             true,
	})
	if err != nil {
		t.Fatalf("CreateMR: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/merge_requests" {
		t.Fatalf("path = %q", escaped)
	}
	if payload["source_branch"] != "feature/x" || payload["target_branch"] != "main" {
		t.Fatalf("payload = %v", payload)
	}
	if payload["remove_source_branch"] != true || payload["squash"] != true {
		t.Fatalf("payload = %v", payload)
	}
	if mr == nil || mr.IID != 10 {
		t.Fatalf("mr = %+v", mr)
	}
}

func TestUpdateMROmitsUnsetFields(t *testing.T) {
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, _ *http.Request, body map[string]any) {
		payload = body
		_, _ = w.Write([]byte(`{"id":100,"iid":9,"title":"new title"}`))
	})

	title := "new title"
	if _, err := client.UpdateMR(context.Background(), "demo/app", 9, UpdateMRInput{Title: &title}); err != nil {
		t.Fatalf("UpdateMR: %v", err)
	}
	if payload["title"] != "new title" {
		t.Fatalf("title = %v", payload["title"])
	}
	if _, exists := payload["description"]; exists {
		t.Fatalf("description must be omitted when unset: %v", payload)
	}
	if _, exists := payload["target_branch"]; exists {
		t.Fatalf("target_branch must be omitted when unset: %v", payload)
	}
}

func TestMRVersions(t *testing.T) {
	var escaped string
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		escaped = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`[{"id":2,"head_commit_sha":"head2","base_commit_sha":"base1","start_commit_sha":"start0"}]`))
	})

	versions, err := client.MRVersions(context.Background(), "demo/app", 9)
	if err != nil {
		t.Fatalf("MRVersions: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/merge_requests/9/versions" {
		t.Fatalf("path = %q", escaped)
	}
	if len(versions) != 1 || versions[0].HeadCommitSHA != "head2" {
		t.Fatalf("versions = %+v", versions)
	}
}

func TestCreateMRMilestone(t *testing.T) {
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, _ *http.Request, body map[string]any) {
		payload = body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":102,"iid":11,"title":"feat: 里程碑"}`))
	})

	if _, err := client.CreateMR(context.Background(), "demo/app", CreateMRInput{
		SourceBranch: "feature/x",
		TargetBranch: "main",
		Title:        "feat: 里程碑",
		MilestoneID:  12,
	}); err != nil {
		t.Fatalf("CreateMR: %v", err)
	}
	if payload["milestone_id"] != float64(12) {
		t.Fatalf("milestone_id = %v", payload["milestone_id"])
	}
}

func TestCreateMROmitsZeroMilestone(t *testing.T) {
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, _ *http.Request, body map[string]any) {
		payload = body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":103,"iid":12,"title":"feat: 无里程碑"}`))
	})

	if _, err := client.CreateMR(context.Background(), "demo/app", CreateMRInput{
		SourceBranch: "feature/y",
		TargetBranch: "main",
		Title:        "feat: 无里程碑",
	}); err != nil {
		t.Fatalf("CreateMR: %v", err)
	}
	if _, exists := payload["milestone_id"]; exists {
		t.Fatalf("milestone_id must be omitted: %v", payload)
	}
}
