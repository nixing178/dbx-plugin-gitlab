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

func newWritableGitLabServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case r.URL.Path == "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/star"):
			_, _ = w.Write([]byte(`{"id":12,"star_count":4,"starred":true}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/unstar"):
			_, _ = w.Write([]byte(`{"id":12,"star_count":3,"starred":false}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/discussions"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"d1","notes":[{"id":31,"body":"LGTM"}]}`))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/merge"):
			_, _ = w.Write([]byte(`{"id":100,"iid":9,"state":"merged"}`))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/merge_requests/9"):
			_, _ = w.Write([]byte(`{"id":100,"iid":9,"state":"closed"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestWriteMethodsBlockedInReadOnlyMode(t *testing.T) {
	server := newGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"iid":          9,
		"body":         "LGTM",
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/commentMr", params); rpcErr == nil {
		t.Fatal("expected read_only error")
	} else if rpcErr.Kind != gitlab.ErrReadOnly {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}
}

func TestCommentAndMergeWhenWritable(t *testing.T) {
	server := newWritableGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	commentParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"iid":          9,
		"body":         "LGTM",
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/commentMr", commentParams)
	if rpcErr != nil {
		t.Fatalf("commentMr: %v", rpcErr)
	}
	note, ok := result.(*gitlab.Note)
	if !ok || note.ID != 31 {
		t.Fatalf("note = %#v", result)
	}

	mergeParams, _ := json.Marshal(map[string]any{
		"connectionId":       "c1",
		"projectId":          "demo/app",
		"iid":                9,
		"squash":             true,
		"removeSourceBranch": true,
		"mergeCommitMessage": "feat: 示例功能",
	})
	merged, rpcErr := service.Handle(context.Background(), "gitlab/mergeMr", mergeParams)
	if rpcErr != nil {
		t.Fatalf("mergeMr: %v", rpcErr)
	}
	mr, ok := merged.(*gitlab.MR)
	if !ok || mr.State != "merged" {
		t.Fatalf("mr = %#v", merged)
	}
}

func TestCloseMRWhenWritable(t *testing.T) {
	server := newWritableGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"iid":          9,
		"stateEvent":   "close",
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/setMrState", params)
	if rpcErr != nil {
		t.Fatalf("setMrState: %v", rpcErr)
	}
	mr, ok := result.(*gitlab.MR)
	if !ok || mr.State != "closed" {
		t.Fatalf("mr = %#v", result)
	}
}
