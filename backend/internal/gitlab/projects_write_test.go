package gitlab

import (
	"context"
	"net/http"
	"testing"
)

func TestStarProjectPostsStar(t *testing.T) {
	var method, escaped string
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		method = r.Method
		escaped = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"id":12,"star_count":4,"starred":true}`))
	})

	result, err := client.StarProject(context.Background(), "demo/app", true)
	if err != nil {
		t.Fatalf("StarProject: %v", err)
	}
	if method != http.MethodPost {
		t.Fatalf("method = %s", method)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/star" {
		t.Fatalf("path = %q", escaped)
	}
	if result == nil || !result.Starred || result.StarCount != 4 {
		t.Fatalf("result = %+v", result)
	}
}

// GitLab 的取消星标是 POST /projects/:id/unstar，没有 DELETE /star（会 404）。
func TestUnstarProjectPostsUnstar(t *testing.T) {
	var method, escaped string
	client := newWriteClient(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		method = r.Method
		escaped = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"id":12,"star_count":2,"starred":false}`))
	})

	result, err := client.StarProject(context.Background(), "demo/app", false)
	if err != nil {
		t.Fatalf("StarProject: %v", err)
	}
	if method != http.MethodPost {
		t.Fatalf("method = %s", method)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/unstar" {
		t.Fatalf("path = %q", escaped)
	}
	if result == nil || result.Starred || result.StarCount != 2 {
		t.Fatalf("result = %+v", result)
	}
}

// 重复加星时 GitLab 返回 304，必须按幂等成功处理，而不是报错。
func TestStarProjectNotModifiedIsIdempotent(t *testing.T) {
	client := newWriteClient(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusNotModified)
	})

	result, err := client.StarProject(context.Background(), "demo/app", true)
	if err != nil {
		t.Fatalf("StarProject: %v", err)
	}
	if result == nil || !result.Starred {
		t.Fatalf("result = %+v", result)
	}
	if result.StarCount != -1 {
		t.Fatalf("star count = %d, want -1 (unknown, keep local count)", result.StarCount)
	}
}
