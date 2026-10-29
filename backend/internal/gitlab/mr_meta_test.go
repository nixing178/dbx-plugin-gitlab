package gitlab

import (
	"context"
	"net/http"
	"testing"
)

// 审批人数：一次 GET 拿批准者、需要数量、剩余数量与「核准为可选」判断依据。
func TestMRApprovalsReadsApprovers(t *testing.T) {
	var escaped string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 900,
			"iid": 128,
			"approvals_required": 2,
			"approvals_left": 1,
			"approved": false,
			"approved_by": [
				{"user": {"id": 2, "username": "user6", "name": "User Six"}}
			]
		}`))
	})

	approvals, err := client.MRApprovals(context.Background(), "demo/project", 128)
	if err != nil {
		t.Fatalf("MRApprovals: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fproject/merge_requests/128/approvals" {
		t.Fatalf("path = %q", escaped)
	}
	if approvals.ApprovalsRequired != 2 || approvals.ApprovalsLeft != 1 || approvals.Approved {
		t.Fatalf("approvals = %+v", approvals)
	}
	if len(approvals.ApprovedBy) != 1 {
		t.Fatalf("approved_by = %+v", approvals.ApprovedBy)
	}
	if approvals.ApprovedBy[0].User == nil || approvals.ApprovedBy[0].User.Username != "user6" {
		t.Fatalf("approver = %+v", approvals.ApprovedBy[0])
	}
}

// 反应列表 / 添加 / 取消三条路径都要带上正确的 method 与 name 参数。
func TestMRAwardEmojiListAddRemove(t *testing.T) {
	var (
		methods []string
		paths   []string
		queries []string
	)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		paths = append(paths, r.URL.EscapedPath())
		queries = append(queries, r.URL.Query().Get("name"))

		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`[{"id":1,"name":"thumbsup","user":{"id":2,"username":"user6"}}]`))
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":5,"name":"thumbsup","user":{"id":1,"username":"user2"}}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	list, err := client.MRAwardEmoji(context.Background(), "demo/project", 128)
	if err != nil {
		t.Fatalf("MRAwardEmoji: %v", err)
	}
	if len(list) != 1 || list[0].Name != "thumbsup" || list[0].User == nil {
		t.Fatalf("awards = %+v", list)
	}

	added, err := client.AddMRAwardEmoji(context.Background(), "demo/project", 128, "thumbsup")
	if err != nil {
		t.Fatalf("AddMRAwardEmoji: %v", err)
	}
	if added == nil || added.ID != 5 {
		t.Fatalf("added = %+v", added)
	}

	if err := client.RemoveMRAwardEmoji(context.Background(), "demo/project", 128, 5); err != nil {
		t.Fatalf("RemoveMRAwardEmoji: %v", err)
	}

	wantMethods := []string{http.MethodGet, http.MethodPost, http.MethodDelete}
	wantPaths := []string{
		"/api/v4/projects/demo%2Fproject/merge_requests/128/award_emoji",
		"/api/v4/projects/demo%2Fproject/merge_requests/128/award_emoji",
		"/api/v4/projects/demo%2Fproject/merge_requests/128/award_emoji/5",
	}
	for index, method := range wantMethods {
		if methods[index] != method {
			t.Fatalf("call %d method = %q, want %q", index, methods[index], method)
		}
		if paths[index] != wantPaths[index] {
			t.Fatalf("call %d path = %q, want %q", index, paths[index], wantPaths[index])
		}
	}
	if queries[1] != "thumbsup" {
		t.Fatalf("add query name = %q", queries[1])
	}
}

// MR 提交必须走 merge request 子资源（只含本 MR 的提交），并带回 X-Total 供页签计数。
func TestMRCommitsReadsScopedCommitsAndTotal(t *testing.T) {
	var (
		escaped string
		query   string
	)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query().Get("per_page")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total", "2")
		_, _ = w.Write([]byte(`[{"id":"abc123","short_id":"abc123","title":"fix"}]`))
	})

	page, err := client.MRCommits(context.Background(), "demo/project", 1235, 1, 30)
	if err != nil {
		t.Fatalf("MRCommits: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fproject/merge_requests/1235/commits" {
		t.Fatalf("path = %q", escaped)
	}
	if query != "30" {
		t.Fatalf("per_page = %q", query)
	}
	if page.Total != 2 {
		t.Fatalf("total = %d, want 2", page.Total)
	}
	if len(page.Items) != 1 || page.Items[0].ShortID != "abc123" {
		t.Fatalf("items = %+v", page.Items)
	}
}

// 里程碑：>0 写入 milestone_id，0 表示清空（GitLab 语义），未传不改。
func TestUpdateMRMilestone(t *testing.T) {
	var payloads []map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, _ *http.Request, body map[string]any) {
		payloads = append(payloads, body)
		_, _ = w.Write([]byte(`{"id":900,"iid":128,"title":"t"}`))
	})

	withMilestone := 7
	if _, err := client.UpdateMR(context.Background(), "demo/app", 128, UpdateMRInput{
		MilestoneID: &withMilestone,
	}); err != nil {
		t.Fatalf("UpdateMR milestone: %v", err)
	}
	if payloads[0]["milestone_id"] != float64(7) {
		t.Fatalf("milestone_id = %v", payloads[0]["milestone_id"])
	}

	clearMilestone := 0
	if _, err := client.UpdateMR(context.Background(), "demo/app", 128, UpdateMRInput{
		MilestoneID: &clearMilestone,
	}); err != nil {
		t.Fatalf("UpdateMR clear milestone: %v", err)
	}
	if payloads[1]["milestone_id"] != float64(0) {
		t.Fatalf("milestone_id = %v", payloads[1]["milestone_id"])
	}
}

// MR 详情要带回里程碑与工时统计（GitLab 详情响应里本来就有）。
func TestMergeRequestCarriesMilestoneAndTimeStats(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 900, "iid": 128, "title": "t",
			"milestone": {"id": 7, "iid": 2, "title": "2026 Q4", "state": "active"},
			"time_stats": {
				"time_estimate": 3600,
				"total_time_spent": 1800,
				"human_time_estimate": "1h",
				"human_total_time_spent": "30m"
			}
		}`))
	})

	mr, err := client.MergeRequest(context.Background(), "demo/app", 128)
	if err != nil {
		t.Fatalf("MergeRequest: %v", err)
	}
	if mr.Milestone == nil || mr.Milestone.Title != "2026 Q4" {
		t.Fatalf("milestone = %+v", mr.Milestone)
	}
	if mr.TimeStats == nil || mr.TimeStats.HumanTotalTimeSpent != "30m" {
		t.Fatalf("time_stats = %+v", mr.TimeStats)
	}
}
