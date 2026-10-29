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

func newExtrasGitLabServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo"}`))
		case r.URL.Path == "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		case strings.HasSuffix(r.URL.Path, "/members/all"):
			_, _ = w.Write([]byte(`[{"id":7,"username":"demo","name":"Demo User","access_level":40}]`))
		case strings.HasSuffix(r.URL.Path, "/search"):
			_, _ = w.Write([]byte(`[{"basename":"order.go","path":"core/order.go","startline":42}]`))
		case strings.HasSuffix(r.URL.Path, "/test_report"):
			_, _ = w.Write([]byte(`{"total_count":2,"success_count":2,"failed_count":0}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestExtrasRPC(t *testing.T) {
	server := newExtrasGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	memberParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"search":       "sample",
	})
	members, rpcErr := service.Handle(context.Background(), "gitlab/projectMembers", memberParams)
	if rpcErr != nil {
		t.Fatalf("projectMembers: %v", rpcErr)
	}
	if page, ok := members.(*gitlab.MemberPage); !ok || len(page.Items) != 1 {
		t.Fatalf("members = %#v", members)
	}

	searchParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"search":       "sampleSymbol",
	})
	results, rpcErr := service.Handle(context.Background(), "gitlab/searchBlobs", searchParams)
	if rpcErr != nil {
		t.Fatalf("searchBlobs: %v", rpcErr)
	}
	if page, ok := results.(*gitlab.SearchPage); !ok || page.Items[0].StartLine != 42 {
		t.Fatalf("results = %#v", results)
	}

	reportParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"pipelineId":   55,
	})
	report, rpcErr := service.Handle(context.Background(), "gitlab/pipelineTestReport", reportParams)
	if rpcErr != nil {
		t.Fatalf("pipelineTestReport: %v", rpcErr)
	}
	if value, ok := report.(*gitlab.TestReport); !ok || value.TotalCount != 2 {
		t.Fatalf("report = %#v", report)
	}
}
