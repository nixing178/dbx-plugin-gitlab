package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/nixing178/dbx-plugin-gitlab/internal/gitlab"
)

func newGitLabServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5","revision":"abc"}`))
		case "/api/v4/projects":
			_, _ = w.Write([]byte(`[{"id":12,"name":"Sample","path_with_namespace":"demo/app"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func connectionPayload(t *testing.T, server *httptest.Server, readOnly bool) json.RawMessage {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	port, _ := strconv.Atoi(parsed.Port())
	payload := map[string]any{
		"connection": map[string]any{
			"id":      "c1",
			"name":    "GitLab",
			"host":    parsed.Hostname(),
			"port":    port,
			"secrets": map[string]any{"token": "test-token"},
			"external_config": map[string]any{
				"scheme":    parsed.Scheme,
				"read_only": readOnly,
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return raw
}

func TestConnectionTestReturnsIdentity(t *testing.T) {
	server := newGitLabServer(t)
	service := NewService()

	result, rpcErr := service.Handle(context.Background(), "connection/test", connectionPayload(t, server, true))
	if rpcErr != nil {
		t.Fatalf("connection/test: %v", rpcErr)
	}
	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if payload["success"] != true {
		t.Fatalf("success = %v", payload["success"])
	}
	if payload["version"] != "17.5.5" {
		t.Fatalf("version = %v", payload["version"])
	}
	if payload["readOnly"] != true {
		t.Fatalf("readOnly = %v", payload["readOnly"])
	}
	user, ok := payload["user"].(*gitlab.User)
	if !ok || user.Username != "demo" {
		t.Fatalf("user = %#v", payload["user"])
	}
}

func TestConnectListAndDisconnect(t *testing.T) {
	server := newGitLabServer(t)
	service := NewService()

	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connection/connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{"connectionId": "c1", "search": "sample", "page": 1, "perPage": 20})
	result, rpcErr := service.Handle(context.Background(), "gitlab/projects", params)
	if rpcErr != nil {
		t.Fatalf("gitlab/projects: %v", rpcErr)
	}
	page, ok := result.(*gitlab.ProjectPage)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if len(page.Items) != 1 || page.Items[0].PathWithNamespace != "demo/app" {
		t.Fatalf("items = %+v", page.Items)
	}

	if _, rpcErr := service.Handle(context.Background(), "connection/disconnect", params); rpcErr != nil {
		t.Fatalf("connection/disconnect: %v", rpcErr)
	}
	if _, rpcErr := service.Handle(context.Background(), "gitlab/projects", params); rpcErr == nil {
		t.Fatal("expected not_connected after disconnect")
	} else if rpcErr.Kind != "not_connected" {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}
}

func TestMissingConnectionId(t *testing.T) {
	service := NewService()
	_, rpcErr := service.Handle(context.Background(), "gitlab/projects", json.RawMessage(`{}`))
	if rpcErr == nil {
		t.Fatal("expected an error")
	}
	if rpcErr.Kind != "invalid_params" {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}
}

func TestUnknownMethod(t *testing.T) {
	service := NewService()
	_, rpcErr := service.Handle(context.Background(), "gitlab/nope", json.RawMessage(`{"connectionId":"c1"}`))
	if rpcErr == nil {
		t.Fatal("expected an error")
	}
	if rpcErr.Kind != "method_not_found" {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}
}
