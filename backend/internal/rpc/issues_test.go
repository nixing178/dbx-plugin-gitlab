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

func newIssuesGitLabServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case r.URL.Path == "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/issues"):
			_, _ = w.Write([]byte(`[{"id":1,"iid":4,"project_id":12,"title":"示例任务失败","state":"opened"}]`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issues"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":2,"iid":5,"title":"新问题","state":"opened"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestIssuesRPC(t *testing.T) {
	server := newIssuesGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"state":        "opened",
		"page":         1,
		"perPage":      20,
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/issues", params)
	if rpcErr != nil {
		t.Fatalf("gitlab/issues: %v", rpcErr)
	}
	page, ok := result.(*gitlab.IssuePage)
	if !ok || len(page.Items) != 1 || page.Items[0].IID != 4 {
		t.Fatalf("page = %#v", result)
	}
}

func TestCreateIssueBlockedInReadOnly(t *testing.T) {
	server := newIssuesGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"title":        "新问题",
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/createIssue", params); rpcErr == nil {
		t.Fatal("expected read_only error")
	} else if rpcErr.Kind != gitlab.ErrReadOnly {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}
}

func TestCreateIssueWhenWritable(t *testing.T) {
	server := newIssuesGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"title":        "新问题",
		"description":  "描述",
		"labels":       []string{"bug"},
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/createIssue", params)
	if rpcErr != nil {
		t.Fatalf("gitlab/createIssue: %v", rpcErr)
	}
	issue, ok := result.(*gitlab.Issue)
	if !ok || issue.IID != 5 {
		t.Fatalf("issue = %#v", result)
	}
}
