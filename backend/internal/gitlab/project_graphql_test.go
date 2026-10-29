package gitlab

import (
	"context"
	"net/http"
	"testing"
)

// REST 的 /projects/:id 与 starred 列表都可能不准，GraphQL 的 viewerHasStarred 是权威来源。
func TestProjectStarredUsesGraphQL(t *testing.T) {
	var graphqlCalled, listCalled bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/api/graphql":
			graphqlCalled = true
			_, _ = w.Write([]byte(`{"data":{"project":{"viewerHasStarred":true}}}`))
		case "/api/v4/projects":
			listCalled = true
			_, _ = w.Write([]byte(`[]`))
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
	if !graphqlCalled {
		t.Fatal("GraphQL viewerHasStarred was not called")
	}
	if listCalled {
		t.Fatal("starred list should not be requested when GraphQL answers")
	}
}

// GraphQL 不可用（老实例 / 被禁用）时退回 starred 列表反查。
func TestProjectStarredFallsBackToStarredList(t *testing.T) {
	var listCalled bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/api/graphql":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		case "/api/v4/projects":
			listCalled = true
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
	if !listCalled {
		t.Fatal("expected fallback to starred list")
	}
}
