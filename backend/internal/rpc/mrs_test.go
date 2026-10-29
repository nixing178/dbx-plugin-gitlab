package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/nixing178/dbx-plugin-gitlab/internal/gitlab"
)

func newMergeRequestsGitLabServer(t *testing.T, onList func(url.Values)) *httptest.Server {
	t.Helper()
	totals := map[string]string{"opened": "0", "merged": "214", "closed": "20", "": "234"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case r.URL.Path == "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		case strings.HasSuffix(r.URL.Path, "/merge_requests"):
			query := r.URL.Query()
			if onList != nil {
				onList(query)
			}
			if total, ok := totals[query.Get("state")]; ok {
				w.Header().Set("X-Total", total)
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestMRCountsRPC(t *testing.T) {
	server := newMergeRequestsGitLabServer(t, nil)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/mrCounts", params)
	if rpcErr != nil {
		t.Fatalf("gitlab/mrCounts: %v", rpcErr)
	}
	counts, ok := result.(*gitlab.MRStateCounts)
	if !ok {
		t.Fatalf("result = %#v", result)
	}
	if counts.Opened != 0 || counts.Merged != 214 || counts.Closed != 20 || counts.All != 234 {
		t.Fatalf("counts = %+v", counts)
	}
}

func TestMergeRequestsRPCPassesSorting(t *testing.T) {
	var list url.Values
	server := newMergeRequestsGitLabServer(t, func(query url.Values) { list = query })
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
		"state":        "opened",
		"orderBy":      "created_at",
		"sort":         "asc",
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/mrs", params); rpcErr != nil {
		t.Fatalf("gitlab/mrs: %v", rpcErr)
	}
	if list.Get("order_by") != "created_at" || list.Get("sort") != "asc" {
		t.Fatalf("list query = %v", list)
	}
}
