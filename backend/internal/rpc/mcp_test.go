package rpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func connectForTest(t *testing.T, service *Service) {
	t.Helper()
	server := newGitLabServer(t)
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connection/connect: %v", rpcErr)
	}
}

func TestMCPToolsListed(t *testing.T) {
	service := NewService()
	connectForTest(t, service)

	result, rpcErr := service.Handle(context.Background(), "mcp/tools", json.RawMessage(`{"connectionId":"c1"}`))
	if rpcErr != nil {
		t.Fatalf("mcp/tools: %v", rpcErr)
	}
	list, ok := result.(MCPToolList)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	names := map[string]bool{}
	for _, tool := range list.Tools {
		names[tool.Name] = true
		if tool.Name == "" || tool.Description == "" || tool.InputSchema == nil {
			t.Fatalf("incomplete tool definition: %+v", tool)
		}
	}
	for _, want := range []string{"gitlab_list_projects", "gitlab_get_project", "gitlab_get_file"} {
		if !names[want] {
			t.Fatalf("missing tool %q in %v", want, names)
		}
	}
}

func TestMCPCallListProjects(t *testing.T) {
	service := NewService()
	connectForTest(t, service)

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"tool":         "gitlab_list_projects",
		"arguments":    map[string]any{"search": "sample"},
	})
	result, rpcErr := service.Handle(context.Background(), "mcp/call", params)
	if rpcErr != nil {
		t.Fatalf("mcp/call: %v", rpcErr)
	}
	callResult, ok := result.(*MCPCallResult)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if callResult.IsError {
		t.Fatalf("unexpected tool error: %+v", callResult.Content)
	}
	if len(callResult.Content) == 0 || !strings.Contains(callResult.Content[0].Text, "demo/app") {
		t.Fatalf("content = %+v", callResult.Content)
	}
}

func TestMCPToolsHideWriteToolsInReadOnlyMode(t *testing.T) {
	service := NewService()
	connectForTest(t, service)

	result, rpcErr := service.Handle(context.Background(), "mcp/tools", json.RawMessage(`{"connectionId":"c1"}`))
	if rpcErr != nil {
		t.Fatalf("mcp/tools: %v", rpcErr)
	}
	list, ok := result.(MCPToolList)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	for _, tool := range list.Tools {
		if writeTools[tool.Name] {
			t.Fatalf("write tool %q must not be exposed in read-only mode", tool.Name)
		}
	}
}

func TestMCPWriteToolBlockedInReadOnlyMode(t *testing.T) {
	service := NewService()
	connectForTest(t, service)

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"tool":         "gitlab_comment_mr",
		"arguments":    map[string]any{"projectId": "demo/app", "iid": 9, "body": "x"},
	})
	result, rpcErr := service.Handle(context.Background(), "mcp/call", params)
	if rpcErr != nil {
		t.Fatalf("mcp/call: %v", rpcErr)
	}
	callResult, ok := result.(*MCPCallResult)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if !callResult.IsError {
		t.Fatalf("expected an error, got %+v", callResult.Content)
	}
}

func TestMCPWriteToolsListedWhenWritable(t *testing.T) {
	service := NewService()
	server := newWritableGitLabServer(t)
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connection/connect: %v", rpcErr)
	}

	result, rpcErr := service.Handle(context.Background(), "mcp/tools", json.RawMessage(`{"connectionId":"c1"}`))
	if rpcErr != nil {
		t.Fatalf("mcp/tools: %v", rpcErr)
	}
	list, ok := result.(MCPToolList)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	names := map[string]bool{}
	for _, tool := range list.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"gitlab_comment_mr", "gitlab_approve_mr", "gitlab_merge_mr", "gitlab_retry_job"} {
		if !names[want] {
			t.Fatalf("missing write tool %q in %v", want, names)
		}
	}
}
