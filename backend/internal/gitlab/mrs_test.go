package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

// GitLab 的全局 MR 接口没有 reviewer scope，必须翻译成 scope=all + reviewer_id。
func TestGlobalMergeRequestsUseScope(t *testing.T) {
	var path string
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4/user":
			_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
		case "/api/v4/merge_requests":
			path = r.URL.Path
			query = r.URL.Query()
			_, _ = w.Write([]byte(`[{"id":1,"iid":9,"title":"feat: 示例功能","state":"opened","source_branch":"feature/x","target_branch":"main"}]`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})

	page, err := client.MergeRequests(context.Background(), "", MROptions{
		Scope:   "reviewer",
		State:   "opened",
		Page:    1,
		PerPage: 20,
	})
	if err != nil {
		t.Fatalf("MergeRequests: %v", err)
	}
	if path != "/api/v4/merge_requests" {
		t.Fatalf("path = %q", path)
	}
	if query.Get("state") != "opened" {
		t.Fatalf("state = %q", query.Get("state"))
	}
	if query.Get("scope") != "all" {
		t.Fatalf("reviewer scope must be sent as scope=all, got %q", query.Get("scope"))
	}
	if query.Get("reviewer_id") != "7" {
		t.Fatalf("reviewer_id = %q", query.Get("reviewer_id"))
	}
	if query.Has("reviewer_username") {
		t.Fatalf("reviewer_username must not be sent: %v", query)
	}
	if len(page.Items) != 1 || page.Items[0].IID != 9 {
		t.Fatalf("items = %+v", page.Items)
	}
}

// 合法 scope 必须原样透传，不能被翻译逻辑误伤。
func TestGlobalMergeRequestsKeepsLegalScopes(t *testing.T) {
	for _, scope := range []string{"created_by_me", "assigned_to_me", "all"} {
		t.Run(scope, func(t *testing.T) {
			var query url.Values
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v4/user" {
					t.Errorf("legal scope %q must not trigger a /user lookup", scope)
					_, _ = w.Write([]byte(`{"id":7,"username":"demo"}`))
					return
				}
				query = r.URL.Query()
				_, _ = w.Write([]byte(`[]`))
			})

			if _, err := client.MergeRequests(context.Background(), "", MROptions{Scope: scope, State: "opened"}); err != nil {
				t.Fatalf("MergeRequests: %v", err)
			}
			if query.Get("scope") != scope {
				t.Fatalf("scope = %q, want %q", query.Get("scope"), scope)
			}
			if query.Has("reviewer_id") {
				t.Fatalf("reviewer_id must not be sent for scope %q: %v", scope, query)
			}
		})
	}
}

func TestProjectMergeRequestsFilters(t *testing.T) {
	var escaped string
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[]`))
	})

	_, err := client.MergeRequests(context.Background(), "demo/app", MROptions{
		State:   "merged",
		Search:  "示例",
		Page:    3,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("MergeRequests: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/merge_requests" {
		t.Fatalf("escaped path = %q", escaped)
	}
	if query.Get("state") != "merged" || query.Get("search") != "示例" {
		t.Fatalf("query = %v", query)
	}
	if query.Get("page") != "3" || query.Get("per_page") != "10" {
		t.Fatalf("paging = %v", query)
	}
	if query.Has("scope") {
		t.Fatalf("scope must not be sent for project-scoped requests: %v", query)
	}
}

func TestMergeRequestDetailParsesNestedFields(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/demo/app/merge_requests/9" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"id":100,"iid":9,"project_id":12,"title":"feat: 示例功能","state":"opened",
			"merge_status":"can_be_merged","detailed_merge_status":"mergeable","has_conflicts":false,
			"source_branch":"feature/x","target_branch":"main","sha":"deadbeef",
			"author":{"id":7,"username":"demo","name":"Demo User"},
			"reviewers":[{"id":8,"username":"user2","name":"User Two"}],
			"labels":["backend"],"user_notes_count":4,"changes_count":"7",
			"pipeline":{"id":55,"status":"success","ref":"feature/x"}
		}`))
	})

	mr, err := client.MergeRequest(context.Background(), "demo/app", 9)
	if err != nil {
		t.Fatalf("MergeRequest: %v", err)
	}
	if mr.IID != 9 || mr.ProjectID != 12 {
		t.Fatalf("ids = %+v", mr)
	}
	if mr.Author == nil || mr.Author.Username != "demo" {
		t.Fatalf("author = %+v", mr.Author)
	}
	if len(mr.Reviewers) != 1 || mr.Reviewers[0].Username != "user2" {
		t.Fatalf("reviewers = %+v", mr.Reviewers)
	}
	if mr.Pipeline == nil || mr.Pipeline.Status != "success" {
		t.Fatalf("pipeline = %+v", mr.Pipeline)
	}
	if mr.DetailedMergeStatus != "mergeable" {
		t.Fatalf("detailed merge status = %q", mr.DetailedMergeStatus)
	}
}

func TestMRDiffsCarriesDiffRefsAndPaging(t *testing.T) {
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("X-Next-Page", "2")
		_, _ = w.Write([]byte(`{
			"diff_refs":{"base_sha":"base1","head_sha":"head2","start_sha":"start3"},
			"changes":[
				{"old_path":"a.go","new_path":"a.go","new_file":false,"diff":"@@ -1,2 +1,3 @@"}
			],
			"overflow":false
		}`))
	})

	page, err := client.MRDiffs(context.Background(), "demo/app", 9, 1, 25)
	if err != nil {
		t.Fatalf("MRDiffs: %v", err)
	}
	if query.Get("per_page") != "25" || query.Get("page") != "1" {
		t.Fatalf("paging = %v", query)
	}
	if page.DiffRefs.HeadSHA != "head2" || page.DiffRefs.BaseSHA != "base1" {
		t.Fatalf("diff refs = %+v", page.DiffRefs)
	}
	if len(page.Items) != 1 || page.Items[0].NewPath != "a.go" {
		t.Fatalf("items = %+v", page.Items)
	}
	if !page.HasMore {
		t.Fatal("has_more should be true when X-Next-Page is present")
	}
}

func TestMRDiscussionsParsesNotesAndPosition(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/demo/app/merge_requests/9/discussions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{
			"id":"d1","individual_note":false,
			"notes":[{
				"id":31,"body":"这里要判空","system":false,"resolvable":true,"resolved":false,
				"author":{"id":8,"username":"user2","name":"User Two"},
				"position":{"base_sha":"b","head_sha":"h","start_sha":"s","new_path":"a.go","new_line":12,"position_type":"text"}
			}]
		}]`))
	})

	discussions, err := client.MRDiscussions(context.Background(), "demo/app", 9)
	if err != nil {
		t.Fatalf("MRDiscussions: %v", err)
	}
	if len(discussions) != 1 || len(discussions[0].Notes) != 1 {
		t.Fatalf("discussions = %+v", discussions)
	}
	note := discussions[0].Notes[0]
	if note.Position == nil || note.Position.NewLine != 12 || note.Position.NewPath != "a.go" {
		t.Fatalf("position = %+v", note.Position)
	}
	if !note.Resolvable || note.Resolved {
		t.Fatalf("resolution flags = resolvable:%v resolved:%v", note.Resolvable, note.Resolved)
	}
}

// CurrentUser 走缓存：翻页时不应每次都多打一次 /user。
func TestCurrentUserIsCached(t *testing.T) {
	calls := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/user" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		calls++
		_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User"}`))
	})

	first, err := client.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	second, err := client.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser (cached): %v", err)
	}
	if first.ID != 7 || second.Username != "demo" {
		t.Fatalf("user = %+v / %+v", first, second)
	}
	if calls != 1 {
		t.Fatalf("/user calls = %d, want 1", calls)
	}
}

// 页签上的计数来自 X-Total，列表结果必须把它带出来。
func TestMergeRequestsCarriesTotal(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total", "234")
		_, _ = w.Write([]byte(`[]`))
	})

	page, err := client.MergeRequests(context.Background(), "demo/project", MROptions{
		State:   "all",
		Page:    1,
		PerPage: 30,
	})
	if err != nil {
		t.Fatalf("MergeRequests: %v", err)
	}
	if page.Total != 234 {
		t.Fatalf("total = %d, want 234", page.Total)
	}
}

// GitLab 网页的排序下拉：order_by / sort 必须透传；未指定时保持实例默认，不多发参数。
func TestMergeRequestsOrderByAndSort(t *testing.T) {
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[]`))
	})

	if _, err := client.MergeRequests(context.Background(), "demo/project", MROptions{
		State:   "opened",
		OrderBy: "created_at",
		Sort:    "asc",
		Page:    1,
		PerPage: 30,
	}); err != nil {
		t.Fatalf("MergeRequests: %v", err)
	}
	if query.Get("order_by") != "created_at" || query.Get("sort") != "asc" {
		t.Fatalf("sorting query = %v", query)
	}

	if _, err := client.MergeRequests(context.Background(), "demo/project", MROptions{
		State:   "opened",
		Page:    1,
		PerPage: 30,
	}); err != nil {
		t.Fatalf("MergeRequests (default sort): %v", err)
	}
	if query.Has("order_by") || query.Has("sort") {
		t.Fatalf("default sorting must not be sent: %v", query)
	}
}
