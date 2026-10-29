package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestCreateBranchPostsBranchAndRef(t *testing.T) {
	var (
		method string
		path   string
		body   map[string]any
	)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.EscapedPath()
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"feature/x","default":false,"protected":true,` +
			`"commit":{"id":"abc123","short_id":"abc123","title":"feat: 分支筛选"}}`))
	})

	branch, err := client.CreateBranch(context.Background(), "demo/app", " feature/x ", " master ")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if method != http.MethodPost || path != "/api/v4/projects/demo%2Fapp/repository/branches" {
		t.Fatalf("request = %s %s", method, path)
	}
	if body["branch"] != "feature/x" || body["ref"] != "master" {
		t.Fatalf("body = %v", body)
	}
	if branch == nil || branch.Name != "feature/x" || !branch.Protected {
		t.Fatalf("branch = %+v", branch)
	}
	if branch.Commit == nil || branch.Commit.ID != "abc123" || branch.Commit.ShortID != "abc123" {
		t.Fatalf("commit = %+v", branch.Commit)
	}
}

func TestCreateBranchRequiresBranchAndRef(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("CreateBranch must not call the API with empty input")
	})

	if _, err := client.CreateBranch(context.Background(), "demo/app", "  ", "master"); !isInvalidConfig(err) {
		t.Fatalf("empty branch err = %v", err)
	}
	if _, err := client.CreateBranch(context.Background(), "demo/app", "feature/x", " "); !isInvalidConfig(err) {
		t.Fatalf("empty ref err = %v", err)
	}
}

func isInvalidConfig(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Kind == ErrInvalidConfig
}
