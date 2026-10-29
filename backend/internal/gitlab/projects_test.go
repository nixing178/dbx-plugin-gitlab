package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestProjectsSendsMembershipAndPagination(t *testing.T) {
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("X-Page", "1")
		w.Header().Set("X-Next-Page", "2")
		w.Header().Set("X-Total", "41")
		_, _ = w.Write([]byte(`[{"id":12,"name":"Sample","path_with_namespace":"demo/app","default_branch":"main","star_count":3}]`))
	})

	page, err := client.Projects(context.Background(), ProjectListOptions{
		Search:  "示例",
		Page:    1,
		PerPage: 20,
	})
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if query.Get("membership") != "true" {
		t.Fatalf("membership = %q", query.Get("membership"))
	}
	if query.Get("order_by") != "last_activity_at" {
		t.Fatalf("order_by = %q", query.Get("order_by"))
	}
	if query.Get("sort") != "desc" {
		t.Fatalf("sort = %q", query.Get("sort"))
	}
	if query.Get("search") != "示例" {
		t.Fatalf("search = %q", query.Get("search"))
	}
	if query.Get("page") != "1" || query.Get("per_page") != "20" {
		t.Fatalf("paging = page %q / per_page %q", query.Get("page"), query.Get("per_page"))
	}
	if len(page.Items) != 1 || page.Items[0].PathWithNamespace != "demo/app" {
		t.Fatalf("items = %+v", page.Items)
	}
	if !page.HasMore {
		t.Fatal("has_more should be true when X-Next-Page is present")
	}
	if page.Total != 41 {
		t.Fatalf("total = %d, want 41", page.Total)
	}
}

func TestProjectsScopesAndOrdering(t *testing.T) {
	cases := []struct {
		name  string
		opts  ProjectListOptions
		check func(t *testing.T, query url.Values)
	}{
		{
			name: "starred scope drops membership",
			opts: ProjectListOptions{Starred: true},
			check: func(t *testing.T, query url.Values) {
				if query.Get("starred") != "true" {
					t.Fatalf("starred = %q", query.Get("starred"))
				}
				if query.Get("membership") != "" {
					t.Fatalf("membership should be empty, got %q", query.Get("membership"))
				}
			},
		},
		{
			name: "owned scope drops membership",
			opts: ProjectListOptions{Owned: true},
			check: func(t *testing.T, query url.Values) {
				if query.Get("owned") != "true" {
					t.Fatalf("owned = %q", query.Get("owned"))
				}
				if query.Get("membership") != "" {
					t.Fatalf("membership should be empty, got %q", query.Get("membership"))
				}
			},
		},
		{
			name: "custom order and direction",
			opts: ProjectListOptions{OrderBy: "name", Sort: "asc"},
			check: func(t *testing.T, query url.Values) {
				if query.Get("order_by") != "name" || query.Get("sort") != "asc" {
					t.Fatalf("order = %q / %q", query.Get("order_by"), query.Get("sort"))
				}
			},
		},
		{
			name: "invalid direction falls back to desc",
			opts: ProjectListOptions{Sort: "sideways"},
			check: func(t *testing.T, query url.Values) {
				if query.Get("sort") != "desc" {
					t.Fatalf("sort = %q", query.Get("sort"))
				}
			},
		},
		{
			name: "inactive filter",
			opts: ProjectListOptions{LastActivityBefore: "2026-04-24T00:00:00Z"},
			check: func(t *testing.T, query url.Values) {
				if query.Get("last_activity_before") != "2026-04-24T00:00:00Z" {
					t.Fatalf("last_activity_before = %q", query.Get("last_activity_before"))
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var query url.Values
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				query = r.URL.Query()
				_, _ = w.Write([]byte(`[]`))
			})
			if _, err := client.Projects(context.Background(), tc.opts); err != nil {
				t.Fatalf("Projects: %v", err)
			}
			tc.check(t, query)
		})
	}
}

func TestProjectEncodesNamespacePath(t *testing.T) {
	var escaped string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		// 详情之后还会试 GraphQL 与 starred=true 反查，这两条返回空即可。
		case "/api/v4/projects":
			_, _ = w.Write([]byte(`[]`))
		case "/api/graphql":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		default:
			escaped = r.URL.EscapedPath()
			_, _ = w.Write([]byte(`{"id":12,"path_with_namespace":"demo/app"}`))
		}
	})

	project, err := client.Project(context.Background(), "demo/app")
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp" {
		t.Fatalf("escaped path = %q", escaped)
	}
	if project.ID != 12 {
		t.Fatalf("id = %d", project.ID)
	}
}

func TestProjectRejectsEmptyID(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := client.Project(context.Background(), "  "); err == nil {
		t.Fatal("expected an error for empty project id")
	}
}
