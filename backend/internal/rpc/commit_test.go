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

func newCommitGitLabServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case r.URL.Path == "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		case strings.HasSuffix(r.URL.Path, "/refs"):
			_, _ = w.Write([]byte(`[{"type":"branch","name":"master"}]`))
		case strings.HasSuffix(r.URL.Path, "/merge_requests"):
			_, _ = w.Write([]byte(`[{"iid":234,"title":"导出功能"}]`))
		case strings.Contains(r.URL.Path, "/repository/commits/"):
			_, _ = w.Write([]byte(`{"id":"4cfa82fc","short_id":"4cfa82fc","title":"Merge branch 'feature' into 'master'"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestCommitDetailRPC(t *testing.T) {
	server := newCommitGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
		"sha":          "4cfa82fc",
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/commitDetail", params)
	if rpcErr != nil {
		t.Fatalf("gitlab/commitDetail: %v", rpcErr)
	}

	detail, ok := result.(*gitlab.CommitDetail)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if detail.Commit == nil || detail.Commit.ShortID != "4cfa82fc" {
		t.Fatalf("commit = %+v", detail.Commit)
	}
	if len(detail.Refs) != 1 || detail.Refs[0].Name != "master" {
		t.Fatalf("refs = %+v", detail.Refs)
	}
	if len(detail.MergeRequests) != 1 || detail.MergeRequests[0].IID != 234 {
		t.Fatalf("merge requests = %+v", detail.MergeRequests)
	}
}

func TestCommitDetailRPCRequiresSHA(t *testing.T) {
	server := newCommitGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/commitDetail", params); rpcErr == nil {
		t.Fatal("expected an invalid params error for a missing sha")
	}
}
