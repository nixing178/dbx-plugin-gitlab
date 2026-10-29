package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nixing178/dbx-plugin-gitlab/internal/gitlab"
)

// 审批与表情回应三个入口都要真的打到 GitLab 对应子资源。
func TestMRApprovalsAndAwardEmojiRPC(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.EscapedPath()+"?name="+r.URL.Query().Get("name"))
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case r.URL.Path == "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		case strings.HasSuffix(r.URL.Path, "/approvals"):
			_, _ = w.Write([]byte(`{"approvals_required":1,"approvals_left":1,"approved":false,"approved_by":[]}`))
		case strings.HasSuffix(r.URL.Path, "/award_emoji") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[{"id":1,"name":"thumbsup","user":{"id":2,"username":"user6"}}]`))
		case strings.HasSuffix(r.URL.Path, "/award_emoji") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":5,"name":"tada"}`))
		case strings.Contains(r.URL.Path, "/award_emoji/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)

	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	base, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
		"iid":          128,
	})

	approvals, rpcErr := service.Handle(context.Background(), "gitlab/mrApprovals", base)
	if rpcErr != nil {
		t.Fatalf("mrApprovals: %v", rpcErr)
	}
	if result, ok := approvals.(*gitlab.MRApprovals); !ok || result.ApprovalsRequired != 1 {
		t.Fatalf("approvals = %#v", approvals)
	}

	list, rpcErr := service.Handle(context.Background(), "gitlab/mrAwardEmoji", base)
	if rpcErr != nil {
		t.Fatalf("mrAwardEmoji: %v", rpcErr)
	}
	if payload, ok := list.(map[string]any); !ok || len(payload["items"].([]gitlab.AwardEmoji)) != 1 {
		t.Fatalf("emoji list = %#v", list)
	}

	addParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
		"iid":          128,
		"emojiName":    "tada",
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/addMrAwardEmoji", addParams); rpcErr != nil {
		t.Fatalf("addMrAwardEmoji: %v", rpcErr)
	}

	removeParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
		"iid":          128,
		"awardId":      5,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/removeMrAwardEmoji", removeParams); rpcErr != nil {
		t.Fatalf("removeMrAwardEmoji: %v", rpcErr)
	}

	joined := strings.Join(calls, "\n")
	for _, want := range []string{
		"GET /api/v4/projects/demo%2Fproject/merge_requests/128/approvals?name=",
		"GET /api/v4/projects/demo%2Fproject/merge_requests/128/award_emoji?name=",
		"POST /api/v4/projects/demo%2Fproject/merge_requests/128/award_emoji?name=tada",
		"DELETE /api/v4/projects/demo%2Fproject/merge_requests/128/award_emoji/5?name=",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing call %q in:\n%s", want, joined)
		}
	}
}

// MR 提交走 merge request 子资源，页签计数依赖 X-Total 透传。
func TestMRCommitsRPC(t *testing.T) {
	var escaped string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case r.URL.Path == "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		case strings.HasSuffix(r.URL.Path, "/merge_requests/1235/commits"):
			escaped = r.URL.EscapedPath()
			w.Header().Set("X-Total", "2")
			_, _ = w.Write([]byte(`[{"id":"abc123","short_id":"abc123","title":"fix"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)

	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
		"iid":          1235,
		"page":         1,
		"perPage":      30,
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/mrCommits", params)
	if rpcErr != nil {
		t.Fatalf("mrCommits: %v", rpcErr)
	}
	page, ok := result.(*gitlab.CommitPage)
	if !ok {
		t.Fatalf("result = %#v", result)
	}
	if page.Total != 2 || len(page.Items) != 1 {
		t.Fatalf("page = %+v", page)
	}
	if escaped != "/api/v4/projects/demo%2Fproject/merge_requests/1235/commits" {
		t.Fatalf("path = %q", escaped)
	}
}

// updateMr 的 milestoneId 要原样进 payload；只读连接必须被挡住。
func TestUpdateMRMilestoneRPC(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case r.URL.Path == "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		case strings.Contains(r.URL.Path, "/merge_requests/128") && r.Method == http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&payload)
			_, _ = w.Write([]byte(`{"id":900,"iid":128,"title":"t"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)

	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
		"iid":          128,
		"milestoneId":  7,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/updateMr", params); rpcErr != nil {
		t.Fatalf("updateMr: %v", rpcErr)
	}
	if payload["milestone_id"] != float64(7) {
		t.Fatalf("payload = %v", payload)
	}

	// 传 0 表示清空里程碑，不能被当成「没传」。
	payload = nil
	clearParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
		"iid":          128,
		"milestoneId":  0,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/updateMr", clearParams); rpcErr != nil {
		t.Fatalf("updateMr clear: %v", rpcErr)
	}
	if payload["milestone_id"] != float64(0) {
		t.Fatalf("clear payload = %v", payload)
	}
}
