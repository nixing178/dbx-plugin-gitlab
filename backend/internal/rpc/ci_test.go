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

func newCIGitLabServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4/user":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case r.URL.Path == "/api/v4/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		case strings.HasSuffix(r.URL.Path, "/jobs/901/trace"):
			w.Header().Set("Content-Range", "bytes 0-4/5")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("hello"))
		case strings.HasSuffix(r.URL.Path, "/jobs/901/retry"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":901,"status":"pending","name":"compile","stage":"build"}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestJobTraceRPC(t *testing.T) {
	server := newCIGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"jobId":        901,
		"offset":       0,
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/jobTrace", params)
	if rpcErr != nil {
		t.Fatalf("jobTrace: %v", rpcErr)
	}
	trace, ok := result.(*gitlab.JobTrace)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if trace.Text != "hello" || trace.NextOffset != 5 || !trace.Complete {
		t.Fatalf("trace = %+v", trace)
	}
}

func TestCIActionsBlockedInReadOnlyMode(t *testing.T) {
	server := newCIGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"jobId":        901,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/retryJob", params); rpcErr == nil {
		t.Fatal("expected read_only error")
	} else if rpcErr.Kind != gitlab.ErrReadOnly {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}
}

func TestRetryJobWhenWritable(t *testing.T) {
	server := newCIGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"jobId":        901,
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/retryJob", params)
	if rpcErr != nil {
		t.Fatalf("retryJob: %v", rpcErr)
	}
	job, ok := result.(*gitlab.Job)
	if !ok || job.ID != 901 || job.Status != "pending" {
		t.Fatalf("job = %#v", result)
	}
}
