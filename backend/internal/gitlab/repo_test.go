package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestTreeSendsPathAndRef(t *testing.T) {
	var query url.Values
	var escaped string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		escaped = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`[{"id":"a1","name":"src","type":"tree","path":"src","mode":"040000"}]`))
	})

	page, err := client.Tree(context.Background(), "demo/app", "src/core", "main", 1, 50)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/repository/tree" {
		t.Fatalf("escaped path = %q", escaped)
	}
	if query.Get("path") != "src/core" || query.Get("ref") != "main" {
		t.Fatalf("query = %v", query)
	}
	if query.Get("per_page") != "50" || query.Get("page") != "1" {
		t.Fatalf("paging = %v", query)
	}
	if len(page.Items) != 1 || page.Items[0].Type != "tree" {
		t.Fatalf("items = %+v", page.Items)
	}
}

func TestFileEncodesRepositoryPath(t *testing.T) {
	var escaped, ref string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		ref = r.URL.Query().Get("ref")
		w.Header().Set("Content-Length", "12")
		_, _ = w.Write([]byte("hello world\n"))
	})

	file, err := client.File(context.Background(), "demo/app", "docs/readme.md", "release/1.2")
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/repository/files/docs%2Freadme.md/raw" {
		t.Fatalf("escaped path = %q", escaped)
	}
	if ref != "release/1.2" {
		t.Fatalf("ref = %q", ref)
	}
	if file.Text != "hello world\n" || file.Size != 12 || file.TooLarge {
		t.Fatalf("file = %+v", file)
	}
}

func TestFileMarksOversizedContent(t *testing.T) {
	server := newTestServerForConfig(t, Config{MaxFileBytes: 4}, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "11")
		_, _ = w.Write([]byte("hello world"))
	})

	file, err := server.File(context.Background(), "demo/app", "big.txt", "main")
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if !file.TooLarge {
		t.Fatalf("expected too_large, got %+v", file)
	}
	if file.Text != "" {
		t.Fatalf("oversized file must not be loaded into memory, got %q", file.Text)
	}
}

func TestCommitsSendsRefAndPagination(t *testing.T) {
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[{"id":"abc123","short_id":"abc123","title":"fix: 修复","author_name":"Demo User","committed_date":"2026-10-01T10:00:00Z"}]`))
	})

	page, err := client.Commits(context.Background(), "demo/app", "feature/x", 2, 30)
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if query.Get("ref_name") != "feature/x" {
		t.Fatalf("ref_name = %q", query.Get("ref_name"))
	}
	if query.Get("page") != "2" || query.Get("per_page") != "30" {
		t.Fatalf("paging = %v", query)
	}
	if len(page.Items) != 1 || page.Items[0].AuthorName != "Demo User" {
		t.Fatalf("items = %+v", page.Items)
	}
}

func TestCommitDiffParsesFlags(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/demo%2Fapp/repository/commits/abc123/diff" &&
			r.URL.EscapedPath() != "/api/v4/projects/demo%2Fapp/repository/commits/abc123/diff" {
			t.Errorf("unexpected path %q", r.URL.EscapedPath())
		}
		_, _ = w.Write([]byte(`[
			{"old_path":"a.txt","new_path":"b.txt","new_file":false,"renamed_file":true,"deleted_file":false,"diff":"@@ -1 +1 @@"},
			{"old_path":"gone.txt","new_path":"gone.txt","new_file":false,"renamed_file":false,"deleted_file":true,"diff":""}
		]`))
	})

	files, err := client.CommitDiff(context.Background(), "demo/app", "abc123")
	if err != nil {
		t.Fatalf("CommitDiff: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d", len(files))
	}
	if !files[0].RenamedFile || files[1].DeletedFile != true {
		t.Fatalf("flags not parsed: %+v", files)
	}
	if !strings.Contains(files[0].Diff, "@@") {
		t.Fatalf("diff body missing: %+v", files[0])
	}
}
