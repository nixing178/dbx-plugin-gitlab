package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

// GitLab 的项目详情不返回 starred，必须用 starred=true 列表反查。
func TestProjectStarredFromList(t *testing.T) {
	var starredQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/api/graphql":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		case "/api/v4/projects":
			starredQuery = r.URL.Query()
			_, _ = w.Write([]byte(`[{"id":12,"path_with_namespace":"demo/app"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	})

	starred, err := client.ProjectStarred(context.Background(), "demo/app")
	if err != nil {
		t.Fatalf("ProjectStarred: %v", err)
	}
	if !starred {
		t.Fatal("starred = false, want true")
	}
	if starredQuery.Get("starred") != "true" {
		t.Fatalf("starred = %q", starredQuery.Get("starred"))
	}
	// GitLab 的 search 只匹配项目名/路径片段，不匹配 "group/project" 完整路径，不能带它。
	if starredQuery.Get("search") != "" {
		t.Fatalf("search = %q, want empty", starredQuery.Get("search"))
	}
	if starredQuery.Get("per_page") != "100" || starredQuery.Get("page") != "1" {
		t.Fatalf("paging = %v", starredQuery)
	}
}

func TestProjectStarredLookupPaginates(t *testing.T) {
	var pages []string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/api/graphql":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		default:
			page := r.URL.Query().Get("page")
			pages = append(pages, page)
			if page == "1" {
				w.Header().Set("X-Next-Page", "2")
				_, _ = w.Write([]byte(`[{"id":99,"path_with_namespace":"other/project"}]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":12,"path_with_namespace":"demo/app"}]`))
		}
	})

	starred, err := client.ProjectStarred(context.Background(), "demo/app")
	if err != nil {
		t.Fatalf("ProjectStarred: %v", err)
	}
	if !starred {
		t.Fatal("starred = false, want true on second page")
	}
	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Fatalf("pages = %v", pages)
	}
}

func TestProjectStarredFalseWhenNotInList(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/api/graphql":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	})

	starred, err := client.ProjectStarred(context.Background(), "demo/app")
	if err != nil {
		t.Fatalf("ProjectStarred: %v", err)
	}
	if starred {
		t.Fatal("starred = true, want false")
	}
}

// 反查失败不能报错拖垮项目页，只按未加星处理。
func TestProjectStarredLookupFailureIsNotFatal(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/api/graphql":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		default:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
		}
	})

	starred, err := client.ProjectStarred(context.Background(), "demo/app")
	if err != nil {
		t.Fatalf("ProjectStarred: %v", err)
	}
	if starred {
		t.Fatal("starred = true, want false")
	}
}
