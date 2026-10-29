package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestProjectLabels(t *testing.T) {
	var escaped string
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"后端","color":"#1f75cb","text_color":"#ffffff","description":"backend"},
			{"id":2,"name":"RPC","color":"#108548","text_color":"#ffffff"}
		]`))
	})

	page, err := client.Labels(context.Background(), "demo/project", " 后端 ", 1, 100)
	if err != nil {
		t.Fatalf("Labels: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fproject/labels" {
		t.Fatalf("path = %q", escaped)
	}
	if query.Get("search") != "后端" {
		t.Fatalf("search = %q", query.Get("search"))
	}
	if query.Get("per_page") != "100" {
		t.Fatalf("per_page = %q", query.Get("per_page"))
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %+v", page.Items)
	}
	if page.Items[0].Name != "后端" || page.Items[0].Color != "#1f75cb" {
		t.Fatalf("label = %+v", page.Items[0])
	}
	if page.Items[1].TextColor != "#ffffff" {
		t.Fatalf("text color = %+v", page.Items[1])
	}
}

func TestProjectMilestones(t *testing.T) {
	var escaped string
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[
			{"id":5,"iid":1,"title":"v1.0","state":"active","due_date":"2026-11-30"},
			{"id":6,"iid":2,"title":"v1.1","state":"active"}
		]`))
	})

	page, err := client.Milestones(context.Background(), "demo/project", "active", 1, 100)
	if err != nil {
		t.Fatalf("Milestones: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fproject/milestones" {
		t.Fatalf("path = %q", escaped)
	}
	if query.Get("state") != "active" {
		t.Fatalf("state = %q", query.Get("state"))
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %+v", page.Items)
	}
	if page.Items[0].ID != 5 || page.Items[0].Title != "v1.0" {
		t.Fatalf("milestone = %+v", page.Items[0])
	}
	if page.Items[0].DueDate != "2026-11-30" {
		t.Fatalf("due date = %q", page.Items[0].DueDate)
	}
}

func TestProjectMilestonesOmitsBlankState(t *testing.T) {
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[]`))
	})

	if _, err := client.Milestones(context.Background(), "demo/project", "  ", 1, 50); err != nil {
		t.Fatalf("Milestones: %v", err)
	}
	if _, exists := query["state"]; exists {
		t.Fatalf("state must be omitted: %v", query)
	}
}
