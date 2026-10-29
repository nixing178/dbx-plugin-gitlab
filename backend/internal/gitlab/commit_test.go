package gitlab

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestCommitDetailLoadsCommitRefsAndMergeRequests(t *testing.T) {
	requested := make([]string, 0, 3)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.EscapedPath())
		switch {
		case strings.HasSuffix(r.URL.Path, "/refs"):
			if r.URL.Query().Get("type") != "all" {
				t.Errorf("refs type = %q", r.URL.Query().Get("type"))
			}
			_, _ = w.Write([]byte(`[{"type":"branch","name":"master"},{"type":"tag","name":"v1.0.0"}]`))
		case strings.HasSuffix(r.URL.Path, "/merge_requests"):
			_, _ = w.Write([]byte(`[{"iid":234,"title":"feat: 示例信息导出","state":"merged"}]`))
		default:
			_, _ = w.Write([]byte(`{
				"id":"4cfa82fc9a1b",
				"short_id":"4cfa82fc",
				"title":"Merge branch 'feature-sample-export' into 'master'",
				"message":"Merge branch 'feature-sample-export' into 'master'\n\nSee merge request 1234",
				"author_name":"user2",
				"parent_ids":["a48935cf7a11"]
			}`))
		}
	})

	detail, err := client.CommitDetail(context.Background(), "demo/project", "4cfa82fc9a1b")
	if err != nil {
		t.Fatalf("CommitDetail: %v", err)
	}
	if detail.Commit == nil || detail.Commit.ShortID != "4cfa82fc" {
		t.Fatalf("commit = %+v", detail.Commit)
	}
	if len(detail.Commit.ParentIDs) != 1 || detail.Commit.ParentIDs[0] != "a48935cf7a11" {
		t.Fatalf("parents = %v", detail.Commit.ParentIDs)
	}
	if len(detail.Refs) != 2 || detail.Refs[0].Type != "branch" || detail.Refs[0].Name != "master" {
		t.Fatalf("refs = %+v", detail.Refs)
	}
	if detail.Refs[1].Type != "tag" || detail.Refs[1].Name != "v1.0.0" {
		t.Fatalf("refs = %+v", detail.Refs)
	}
	if len(detail.MergeRequests) != 1 || detail.MergeRequests[0].IID != 234 {
		t.Fatalf("merge requests = %+v", detail.MergeRequests)
	}
	if len(requested) != 3 {
		t.Fatalf("requested %d endpoints: %v", len(requested), requested)
	}
}

// refs / merge_requests 是「锦上添花」：老版本或权限不足只降级，不能让详情页整体失败。
func TestCommitDetailDegradesWhenRefsFail(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/refs") || strings.HasSuffix(r.URL.Path, "/merge_requests") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"4cfa82fc","short_id":"4cfa82fc","title":"Initial commit"}`))
	})

	detail, err := client.CommitDetail(context.Background(), "demo/project", "4cfa82fc")
	if err != nil {
		t.Fatalf("CommitDetail should degrade: %v", err)
	}
	if detail.Commit == nil || detail.Commit.Title != "Initial commit" {
		t.Fatalf("commit = %+v", detail.Commit)
	}
	if len(detail.Refs) != 0 || len(detail.MergeRequests) != 0 {
		t.Fatalf("refs/mrs should stay empty: %+v %+v", detail.Refs, detail.MergeRequests)
	}
}

func TestCommitDetailRequiresSHA(t *testing.T) {
	client := newTestClient(t, func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("no request expected for an empty sha")
	})

	if _, err := client.CommitDetail(context.Background(), "demo/project", "  "); err == nil {
		t.Fatal("expected an error for a missing sha")
	}
}
