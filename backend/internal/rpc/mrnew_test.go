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

// newMrNewGitLabServer 模拟「新建合并请求」页会打到的只读接口与创建接口。
func newMrNewGitLabServer(
	t *testing.T,
	onCreate func(body map[string]any),
) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case r.URL.Path == "/api/v4/version":
			_, _ = w.Write([]byte(`{"version":"17.5.5"}`))
		case strings.HasSuffix(r.URL.Path, "/repository/compare"):
			_, _ = w.Write([]byte(`{
				"commits":[{"id":"dcCBFbEf","short_id":"dcCBFbEf","title":"refactor(sample): 移除示例接口"}],
				"diffs":[{"old_path":"a.kt","new_path":"a.kt","diff":"@@ -1 +1,2 @@\n-old\n+new\n+more"}],
				"compare_timeout":false,
				"compare_same_ref":false
			}`))
		case strings.HasSuffix(r.URL.Path, "/labels"):
			_, _ = w.Write([]byte(`[{"id":1,"name":"后端","color":"#1f75cb","text_color":"#ffffff"}]`))
		case strings.HasSuffix(r.URL.Path, "/milestones"):
			_, _ = w.Write([]byte(`[{"id":5,"iid":1,"title":"v1.0","state":"active"}]`))
		case strings.HasSuffix(r.URL.Path, "/merge_requests") && r.Method == http.MethodPost:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if onCreate != nil {
				onCreate(body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":1,"iid":2,"title":"refactor(sample)"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func connectReadOnly(t *testing.T, server *httptest.Server) *Service {
	t.Helper()
	service := NewService()
	if _, rpcErr := service.Handle(
		context.Background(),
		"connection/connect",
		connectionPayload(t, server, true),
	); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}
	return service
}

func TestCompareRPC(t *testing.T) {
	server := newMrNewGitLabServer(t, nil)
	service := connectReadOnly(t, server)

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
		"from":         "master",
		"to":           "feature/sample-change",
	})
	result, rpcErr := service.Handle(context.Background(), "gitlab/compare", params)
	if rpcErr != nil {
		t.Fatalf("gitlab/compare: %v", rpcErr)
	}
	compare, ok := result.(*gitlab.CompareResult)
	if !ok {
		t.Fatalf("result = %#v", result)
	}
	if len(compare.Commits) != 1 || compare.Commits[0].ShortID != "dcCBFbEf" {
		t.Fatalf("commits = %+v", compare.Commits)
	}
	if len(compare.Diffs) != 1 || compare.Diffs[0].Additions != 2 {
		t.Fatalf("diffs = %+v", compare.Diffs)
	}
}

func TestCompareRPCNeedsBothRefs(t *testing.T) {
	server := newMrNewGitLabServer(t, nil)
	service := connectReadOnly(t, server)

	params, _ := json.Marshal(map[string]any{
		"connectionId": "c1",
		"projectId":    "demo/project",
		"from":         "master",
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/compare", params); rpcErr == nil {
		t.Fatal("expected an invalid params error")
	}
}

func TestLabelsAndMilestonesRPC(t *testing.T) {
	server := newMrNewGitLabServer(t, nil)
	service := connectReadOnly(t, server)

	base := map[string]any{"connectionId": "c1", "projectId": "demo/project"}

	labelsParams, _ := json.Marshal(base)
	labelsResult, rpcErr := service.Handle(context.Background(), "gitlab/labels", labelsParams)
	if rpcErr != nil {
		t.Fatalf("gitlab/labels: %v", rpcErr)
	}
	labels, ok := labelsResult.(*gitlab.LabelPage)
	if !ok {
		t.Fatalf("labels result = %#v", labelsResult)
	}
	if len(labels.Items) != 1 || labels.Items[0].Name != "后端" {
		t.Fatalf("labels = %+v", labels.Items)
	}

	milestoneParams, _ := json.Marshal(base)
	milestonesResult, rpcErr := service.Handle(
		context.Background(),
		"gitlab/milestones",
		milestoneParams,
	)
	if rpcErr != nil {
		t.Fatalf("gitlab/milestones: %v", rpcErr)
	}
	milestones, ok := milestonesResult.(*gitlab.MilestonePage)
	if !ok {
		t.Fatalf("milestones result = %#v", milestonesResult)
	}
	if len(milestones.Items) != 1 || milestones.Items[0].ID != 5 {
		t.Fatalf("milestones = %+v", milestones.Items)
	}
}

func TestCreateMrRPCPassesMilestone(t *testing.T) {
	var body map[string]any
	server := newMrNewGitLabServer(t, func(received map[string]any) { body = received })

	service := NewService()
	if _, rpcErr := service.Handle(
		context.Background(),
		"connection/connect",
		connectionPayload(t, server, false),
	); rpcErr != nil {
		t.Fatalf("connect: %v", rpcErr)
	}

	params, _ := json.Marshal(map[string]any{
		"connectionId":       "c1",
		"projectId":          "demo/project",
		"sourceBranch":       "feature/sample-change",
		"targetBranch":       "master",
		"title":              "refactor(sample): 调整示例服务调用",
		"description":        "- 新增示例配置",
		"milestoneId":        12,
		"labels":             []string{"后端"},
		"assigneeIds":        []int{7},
		"reviewerIds":        []int{9},
		"removeSourceBranch": true,
		"squash":             false,
	})
	if _, rpcErr := service.Handle(context.Background(), "gitlab/createMr", params); rpcErr != nil {
		t.Fatalf("gitlab/createMr: %v", rpcErr)
	}
	if body["milestone_id"] != float64(12) {
		t.Fatalf("milestone_id = %v", body["milestone_id"])
	}
	if body["labels"] != "后端" {
		t.Fatalf("labels = %v", body["labels"])
	}
	if body["remove_source_branch"] != true || body["squash"] != false {
		t.Fatalf("merge options = %v", body)
	}
	if body["source_branch"] != "feature/sample-change" {
		t.Fatalf("source_branch = %v", body["source_branch"])
	}
}
