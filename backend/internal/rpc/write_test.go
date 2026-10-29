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
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/repository/branches"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name":"feature/new","default":false,"protected":false}`))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/merge"):
			_, _ = w.Write([]byte(`{"id":100,"iid":9,"state":"merged"}`))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/merge_requests/9"):
			_, _ = w.Write([]byte(`{"id":100,"iid":9,"state":"closed"}`))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/members/8"):
			_, _ = w.Write([]byte(`{"id":8,"username":"demo","name":"Demo User","access_level":40}`))
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/members/8"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/members/all/7"):
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User","access_level":40}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/users"):
			_, _ = w.Write([]byte(`[{"id":8,"username":"user3","name":"测试用户","state":"active"}]`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/members"):
			_, _ = w.Write([]byte(`{"id":8,"username":"user3","access_level":30}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/invitations"):
			_, _ = w.Write([]byte(`{"id":9,"username":"invitee","access_level":10}`))
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

	branchParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"branch":       "feature/x",
		"ref":          "master",
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/createBranch", branchParams); rpcErr == nil {
		t.Fatal("expected read_only error for createBranch")
	} else if rpcErr.Kind != gitlab.ErrReadOnly {
		t.Fatalf("createBranch kind = %q", rpcErr.Kind)
	}
}

func TestCreateBranchWhenWritable(t *testing.T) {
	server := newWritableGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"branch":       "feature/new",
		"ref":          "master",
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/createBranch", params)
	if rpcErr != nil {
		t.Fatalf("createBranch: %v", rpcErr)
	}
	branch, ok := result.(*gitlab.Branch)
	if !ok || branch.Name != "feature/new" {
		t.Fatalf("branch = %#v", result)
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

func TestMemberExpiryWrites(t *testing.T) {
	readOnlyServer := newGitLabServer(t)
	readOnlyService := NewService()
	if _, rpcErr := readOnlyService.Handle(context.Background(), "connection/connect", connectionPayload(t, readOnlyServer, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"userId":       8,
		"accessLevel":  30,
		"expiresAt":    "2026-12-31",
	})
	if _, rpcErr := readOnlyService.Handle(context.Background(), "gitlab/updateMemberExpiry", params); rpcErr == nil {
		t.Fatal("expected read_only error")
	} else if rpcErr.Kind != gitlab.ErrReadOnly {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}

	server := newWritableGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}
	result, rpcErr := service.Handle(context.Background(), "gitlab/updateMemberExpiry", params)
	if rpcErr != nil {
		t.Fatalf("updateMemberExpiry: %v", rpcErr)
	}
	member, ok := result.(*gitlab.Member)
	if !ok || member.Source != "direct" {
		t.Fatalf("member = %#v", result)
	}
}

func TestProjectMemberRPC(t *testing.T) {
	server := newWritableGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"userId":       7,
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/projectMember", params)
	if rpcErr != nil {
		t.Fatalf("projectMember: %v", rpcErr)
	}
	member, ok := result.(*gitlab.Member)
	if !ok || member.AccessLevel != 40 {
		t.Fatalf("member = %#v", result)
	}
}

func TestInviteMemberWritesBlockedInReadOnlyMode(t *testing.T) {
	server := newGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	addParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"username":     "user3",
		"accessLevel":  30,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/addMember", addParams); rpcErr == nil {
		t.Fatal("expected read_only error")
	} else if rpcErr.Kind != gitlab.ErrReadOnly {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}

	inviteParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"email":        "invitee@example.com",
		"accessLevel":  10,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/inviteMember", inviteParams); rpcErr == nil {
		t.Fatal("expected read_only error")
	} else if rpcErr.Kind != gitlab.ErrReadOnly {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}
}

func TestInviteMemberWritesWhenWritable(t *testing.T) {
	server := newWritableGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	addParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"username":     "user3",
		"accessLevel":  30,
		"expiresAt":    "2026-11-19",
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/addMember", addParams)
	if rpcErr != nil {
		t.Fatalf("addMember: %v", rpcErr)
	}
	member, ok := result.(*gitlab.Member)
	if !ok || member.Username != "user3" || member.Source != "direct" {
		t.Fatalf("member = %#v", result)
	}

	inviteParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"email":        "invitee@example.com",
		"accessLevel":  10,
	})
	invited, rpcErr := service.Handle(context.Background(), "gitlab/inviteMember", inviteParams)
	if rpcErr != nil {
		t.Fatalf("inviteMember: %v", rpcErr)
	}
	if invited.(*gitlab.Member).Username != "invitee" {
		t.Fatalf("invited = %#v", invited)
	}
}

func TestSearchUsersRPC(t *testing.T) {
	server := newWritableGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"search":       "测试",
		"perPage":      20,
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/searchUsers", params)
	if rpcErr != nil {
		t.Fatalf("searchUsers: %v", rpcErr)
	}
	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v", result)
	}
	users, ok := payload["items"].([]gitlab.User)
	if !ok || len(users) != 1 || users[0].Username != "user3" {
		t.Fatalf("items = %#v", payload["items"])
	}
}

func TestMemberWritesBlockedInReadOnlyMode(t *testing.T) {
	server := newGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, true)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	roleParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"userId":       8,
		"accessLevel":  40,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/updateMemberRole", roleParams); rpcErr == nil {
		t.Fatal("expected read_only error")
	} else if rpcErr.Kind != gitlab.ErrReadOnly {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}

	removeParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"userId":       8,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/removeMember", removeParams); rpcErr == nil {
		t.Fatal("expected read_only error")
	} else if rpcErr.Kind != gitlab.ErrReadOnly {
		t.Fatalf("kind = %q", rpcErr.Kind)
	}
}

func TestMemberWritesWhenWritable(t *testing.T) {
	server := newWritableGitLabServer(t)
	service := NewService()
	if _, rpcErr := service.Handle(context.Background(), "connection/connect", connectionPayload(t, server, false)); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	roleParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"userId":       8,
		"accessLevel":  40,
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/updateMemberRole", roleParams)
	if rpcErr != nil {
		t.Fatalf("updateMemberRole: %v", rpcErr)
	}
	member, ok := result.(*gitlab.Member)
	if !ok || member.AccessLevel != 40 || member.Source != "direct" {
		t.Fatalf("member = %#v", result)
	}

	removeParams, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/app",
		"userId":       8,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/removeMember", removeParams); rpcErr != nil {
		t.Fatalf("removeMember: %v", rpcErr)
	}
}
