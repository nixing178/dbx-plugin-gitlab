package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestIssuesProjectScope(t *testing.T) {
	var escaped string
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[{"id":1,"iid":4,"title":"示例任务失败","state":"opened","project_id":12}]`))
	})

	page, err := client.Issues(context.Background(), IssueOptions{
		ProjectID: "demo/app",
		State:     "opened",
		Search:    "示例任务",
		Page:      1,
		PerPage:   20,
	})
	if err != nil {
		t.Fatalf("Issues: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/issues" {
		t.Fatalf("path = %q", escaped)
	}
	if query.Get("state") != "opened" || query.Get("search") != "示例任务" {
		t.Fatalf("query = %v", query)
	}
	if query.Has("scope") {
		t.Fatalf("scope must not be sent for project issues: %v", query)
	}
	if len(page.Items) != 1 || page.Items[0].IID != 4 {
		t.Fatalf("items = %+v", page.Items)
	}
}

func TestIssuesGlobalScope(t *testing.T) {
	var escaped string
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[]`))
	})

	if _, err := client.Issues(context.Background(), IssueOptions{
		Scope:   "assigned_to_me",
		State:   "opened",
		Page:    2,
		PerPage: 10,
	}); err != nil {
		t.Fatalf("Issues: %v", err)
	}
	if escaped != "/api/v4/issues" {
		t.Fatalf("path = %q", escaped)
	}
	if query.Get("scope") != "assigned_to_me" {
		t.Fatalf("scope = %q", query.Get("scope"))
	}
	if query.Get("page") != "2" || query.Get("per_page") != "10" {
		t.Fatalf("paging = %v", query)
	}
}

func TestIssueDetailAndNotes(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/api/v4/projects/demo%2Fapp/issues/4":
			_, _ = w.Write([]byte(`{
				"id":1,"iid":4,"project_id":12,"title":"示例任务失败","state":"opened",
				"description":"复现步骤","labels":["bug"],"user_notes_count":2,
				"author":{"id":7,"username":"demo"}
			}`))
		case "/api/v4/projects/demo%2Fapp/issues/4/notes":
			_, _ = w.Write([]byte(`[{"id":11,"body":"已定位","author":{"id":8,"username":"user2"}}]`))
		default:
			t.Errorf("unexpected path %q", r.URL.EscapedPath())
		}
	})

	issue, err := client.Issue(context.Background(), "demo/app", 4)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if issue.Title != "示例任务失败" || len(issue.Labels) != 1 {
		t.Fatalf("issue = %+v", issue)
	}

	notes, err := client.IssueNotes(context.Background(), "demo/app", 4)
	if err != nil {
		t.Fatalf("IssueNotes: %v", err)
	}
	if len(notes) != 1 || notes[0].Body != "已定位" {
		t.Fatalf("notes = %+v", notes)
	}
}

func TestCreateIssue(t *testing.T) {
	var method, escaped string
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		method = r.Method
		escaped = r.URL.EscapedPath()
		payload = body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":2,"iid":5,"title":"新问题"}`))
	})

	issue, err := client.CreateIssue(context.Background(), "demo/app", CreateIssueInput{
		Title:       "新问题",
		Description: "描述",
		Labels:      []string{"bug", "urgent"},
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if method != http.MethodPost || escaped != "/api/v4/projects/demo%2Fapp/issues" {
		t.Fatalf("%s %s", method, escaped)
	}
	if payload["title"] != "新问题" || payload["labels"] != "bug,urgent" {
		t.Fatalf("payload = %v", payload)
	}
	if issue == nil || issue.IID != 5 {
		t.Fatalf("issue = %+v", issue)
	}
}

func TestUpdateIssueStateEvent(t *testing.T) {
	var method string
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		method = r.Method
		payload = body
		_, _ = w.Write([]byte(`{"id":1,"iid":4,"state":"closed"}`))
	})

	issue, err := client.UpdateIssue(context.Background(), "demo/app", 4, UpdateIssueInput{
		StateEvent: "close",
	})
	if err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if method != http.MethodPut {
		t.Fatalf("method = %s", method)
	}
	if payload["state_event"] != "close" {
		t.Fatalf("payload = %v", payload)
	}
	if issue.State != "closed" {
		t.Fatalf("issue = %+v", issue)
	}
}

func TestCommentIssue(t *testing.T) {
	var escaped string
	var payload map[string]any
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		escaped = r.URL.EscapedPath()
		payload = body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":12,"body":"收到"}`))
	})

	note, err := client.CommentIssue(context.Background(), "demo/app", 4, "收到")
	if err != nil {
		t.Fatalf("CommentIssue: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/issues/4/notes" {
		t.Fatalf("path = %q", escaped)
	}
	if payload["body"] != "收到" || note.ID != 12 {
		t.Fatalf("payload = %v, note = %+v", payload, note)
	}
}

func TestTodosAndMarkDone(t *testing.T) {
	var calls []string
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		calls = append(calls, r.Method+" "+r.URL.EscapedPath()+"?"+r.URL.RawQuery)
		if r.URL.Path == "/api/v4/todos" {
			_, _ = w.Write([]byte(`[{"id":9,"action_name":"assigned","target_type":"Issue","state":"pending","target":{"iid":4,"title":"示例任务失败"}}]`))
			return
		}
		_, _ = w.Write([]byte(`{"id":9,"state":"done"}`))
	})

	page, err := client.Todos(context.Background(), "pending", 1, 20)
	if err != nil {
		t.Fatalf("Todos: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Target == nil || page.Items[0].Target.IID != 4 {
		t.Fatalf("items = %+v", page.Items)
	}
	if calls[0] != "GET /api/v4/todos?page=1&per_page=20&state=pending" {
		t.Fatalf("call = %q", calls[0])
	}

	todo, err := client.MarkTodoDone(context.Background(), 9)
	if err != nil {
		t.Fatalf("MarkTodoDone: %v", err)
	}
	if todo.State != "done" {
		t.Fatalf("todo = %+v", todo)
	}
	if calls[1] != "POST /api/v4/todos/9/mark_as_done?" {
		t.Fatalf("call = %q", calls[1])
	}
}
