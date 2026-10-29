package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nixing178/dbx-plugin-gitlab/internal/gitlab"
)

func TestStarProjectWhenWritable(t *testing.T) {
	server := newWritableGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"starred":      true,
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/starProject", params)
	if rpcErr != nil {
		t.Fatalf("starProject: %v", rpcErr)
	}
	star, ok := result.(*gitlab.StarResult)
	if !ok || !star.Starred || star.StarCount != 4 {
		t.Fatalf("result = %#v", result)
	}
}

func TestStarProjectBlockedInReadOnlyMode(t *testing.T) {
	server := newGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"starred":      true,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/starProject", params); rpcErr == nil {
		t.Fatal("expected read_only error")
	} else if rpcErr.Kind != gitlab.ErrReadOnly {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}
}
