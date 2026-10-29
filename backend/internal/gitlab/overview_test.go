package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestBranchesSendsSearchAndDecodes(t *testing.T) {
	var (
		mu    sync.Mutex
		query url.Values
	)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		query = r.URL.Query()
		mu.Unlock()
		_, _ = w.Write([]byte(`[{"name":"main","default":true,"protected":true,"commit":{"id":"abc","short_id":"abc"}}]`))
	})

	page, err := client.Branches(context.Background(), "demo/app", "feat", 1, 100)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if query.Get("search") != "feat" || query.Get("per_page") != "100" {
		t.Fatalf("query = %v", query)
	}
	if len(page.Items) != 1 || page.Items[0].Name != "main" || !page.Items[0].Default {
		t.Fatalf("items = %+v", page.Items)
	}
	if page.Items[0].Commit == nil || page.Items[0].Commit.ID != "abc" {
		t.Fatalf("commit = %+v", page.Items[0].Commit)
	}
}

func TestRepoOverviewAggregatesCountsAndHead(t *testing.T) {
	var headRef string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/repository/branches"):
			w.Header().Set("X-Total", "188")
			_, _ = w.Write([]byte(`[]`))
		case strings.HasSuffix(r.URL.Path, "/repository/tags"):
			w.Header().Set("X-Total", "0")
			_, _ = w.Write([]byte(`[]`))
		case strings.HasSuffix(r.URL.Path, "/repository/commits"):
			headRef = r.URL.Query().Get("ref_name")
			_, _ = w.Write([]byte(`[{"id":"head1","short_id":"head1","title":"Merge branch"}]`))
		default:
			_, _ = w.Write([]byte(`{
				"id": 7,
				"name": "sample",
				"default_branch": "master",
				"created_at": "2024-09-27T08:00:00Z",
				"statistics": {"commit_count": 2886, "storage_size": 22754099, "repository_size": 20000000}
			}`))
		}
	})

	overview, err := client.RepoOverview(context.Background(), "demo/app", "")
	if err != nil {
		t.Fatalf("RepoOverview: %v", err)
	}
	if overview.BranchCount != 188 || overview.TagCount != 0 {
		t.Fatalf("counts = %d/%d", overview.BranchCount, overview.TagCount)
	}
	if overview.CommitCount != 2886 || overview.StorageSize != 22754099 {
		t.Fatalf("stats = %+v", overview)
	}
	if overview.CreatedAt.IsZero() || overview.CreatedAt.Year() != 2024 {
		t.Fatalf("created_at = %v", overview.CreatedAt)
	}
	if headRef != "master" {
		t.Fatalf("head ref = %q", headRef)
	}
	if overview.HeadCommit == nil || overview.HeadCommit.ID != "head1" {
		t.Fatalf("head commit = %+v", overview.HeadCommit)
	}
}

func TestRepoOverviewKeepsUnknownCounts(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/repository/") {
			_, _ = w.Write([]byte(`{"id":7,"default_branch":"main"}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/repository/commits") {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		// 分支/标签端点没有 X-Total 时后端不应伪装成 0。
		_, _ = w.Write([]byte(`[]`))
	})

	overview, err := client.RepoOverview(context.Background(), "demo/app", "release")
	if err != nil {
		t.Fatalf("RepoOverview: %v", err)
	}
	if overview.BranchCount != -1 || overview.TagCount != -1 {
		t.Fatalf("counts = %d/%d", overview.BranchCount, overview.TagCount)
	}
	if overview.HeadCommit != nil {
		t.Fatalf("head commit = %+v", overview.HeadCommit)
	}
}

func TestTreeCommitsKeepsOrderAndSkipsBrokenPath(t *testing.T) {
	var mu sync.Mutex
	requested := map[string]int{}
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		mu.Lock()
		requested[path]++
		mu.Unlock()

		if path == "broken" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"commit-` + path + `","short_id":"c","title":"` + path + `"}]`))
	})

	items, err := client.TreeCommits(
		context.Background(),
		"demo/app",
		"main",
		[]string{"src", "broken", "src", "/docs/"},
	)
	if err != nil {
		t.Fatalf("TreeCommits: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %+v", items)
	}
	if items[0].Path != "src" || items[0].Commit == nil || items[0].Commit.ID != "commit-src" {
		t.Fatalf("items[0] = %+v", items[0])
	}
	if items[1].Path != "broken" || items[1].Commit != nil {
		t.Fatalf("items[1] = %+v", items[1])
	}
	if items[2].Path != "docs" || items[2].Commit == nil {
		t.Fatalf("items[2] = %+v", items[2])
	}

	mu.Lock()
	defer mu.Unlock()
	if requested["src"] != 1 {
		t.Fatalf("duplicate path requested %d times", requested["src"])
	}
}

func TestTreeCommitsCapsPathCount(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"x","short_id":"x","title":"x"}]`))
	})

	paths := make([]string, 0, 80)
	for index := 0; index < 80; index++ {
		paths = append(paths, "dir/file-"+strings.Repeat("a", index%3)+string(rune('a'+index%26))+string(rune('a'+index/26)))
	}
	items, err := client.TreeCommits(context.Background(), "demo/app", "main", paths)
	if err != nil {
		t.Fatalf("TreeCommits: %v", err)
	}
	if len(items) > maxTreeCommitPaths {
		t.Fatalf("len(items) = %d", len(items))
	}
}
