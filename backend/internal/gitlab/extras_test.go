package gitlab

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestProjectMembers(t *testing.T) {
	paths := make([]string, 0, 2)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.EscapedPath())
		switch {
		case strings.HasSuffix(r.URL.Path, "/members/all"):
			if r.URL.Query().Get("query") != "sample" {
				t.Errorf("query = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`[
				{"id":7,"username":"demo","name":"Demo User","access_level":40,"state":"active","created_at":"2026-09-24T10:00:00Z"},
				{"id":8,"username":"inherited","name":"Inherited User","access_level":30,"state":"active"}
			]`))
		case strings.HasSuffix(r.URL.Path, "/members"):
			_, _ = w.Write([]byte(`[{"id":7,"username":"demo","name":"Demo User","access_level":40}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		}
	})

	page, err := client.ProjectMembers(context.Background(), "demo/app", "sample", 1, 50)
	if err != nil {
		t.Fatalf("ProjectMembers: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/api/v4/projects/demo%2Fapp/members/all" ||
		paths[1] != "/api/v4/projects/demo%2Fapp/members" {
		t.Fatalf("paths = %v", paths)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %+v", page.Items)
	}
	if page.Items[0].Source != "direct" || page.Items[1].Source != "inherited" {
		t.Fatalf("sources = %q / %q", page.Items[0].Source, page.Items[1].Source)
	}
	if page.Items[0].CreatedAt != "2026-09-24T10:00:00Z" {
		t.Fatalf("created_at = %q", page.Items[0].CreatedAt)
	}
}

func TestProjectMembersSourceUnknownWhenDirectListFails(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/members/all") {
			_, _ = w.Write([]byte(`[{"id":7,"username":"demo","access_level":40}]`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
	})

	page, err := client.ProjectMembers(context.Background(), "demo/app", "", 1, 50)
	if err != nil {
		t.Fatalf("ProjectMembers: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Source != "" {
		t.Fatalf("items = %+v", page.Items)
	}
}

func TestProjectMembersDirectListFollowsPaging(t *testing.T) {
	directPages := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/members/all") {
			_, _ = w.Write([]byte(`[
				{"id":7,"username":"demo","access_level":40},
				{"id":9,"username":"page2","access_level":30}
			]`))
			return
		}
		directPages++
		if directPages == 1 {
			w.Header().Set("X-Page", "1")
			w.Header().Set("X-Next-Page", "2")
			_, _ = w.Write([]byte(`[{"id":7,"username":"demo","access_level":40}]`))
			return
		}
		w.Header().Set("X-Page", "2")
		_, _ = w.Write([]byte(`[{"id":9,"username":"page2","access_level":30}]`))
	})

	page, err := client.ProjectMembers(context.Background(), "demo/app", "", 1, 50)
	if err != nil {
		t.Fatalf("ProjectMembers: %v", err)
	}
	if directPages != 2 {
		t.Fatalf("direct pages = %d", directPages)
	}
	for _, item := range page.Items {
		if item.Source != "direct" {
			t.Fatalf("item %d source = %q", item.ID, item.Source)
		}
	}
}

func TestProjectMember(t *testing.T) {
	var escaped string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"id":7,"username":"demo","name":"Demo User","access_level":40}`))
	})

	member, err := client.ProjectMember(context.Background(), "demo/app", 7)
	if err != nil {
		t.Fatalf("ProjectMember: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/members/all/7" {
		t.Fatalf("path = %q", escaped)
	}
	if member.AccessLevel != 40 {
		t.Fatalf("member = %+v", member)
	}
}

func TestProjectMemberValidatesUserID(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := client.ProjectMember(context.Background(), "demo/app", 0); err == nil {
		t.Fatal("expected an error for a missing user id")
	}
}

func TestUpdateMemberRole(t *testing.T) {
	var method, escaped string
	var payload map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		escaped = r.URL.EscapedPath()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":8,"username":"demo","name":"Demo User","access_level":40}`))
	})

	member, err := client.UpdateMemberRole(context.Background(), "demo/app", 8, 40)
	if err != nil {
		t.Fatalf("UpdateMemberRole: %v", err)
	}
	if method != http.MethodPut {
		t.Fatalf("method = %q", method)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/members/8" {
		t.Fatalf("path = %q", escaped)
	}
	if payload["access_level"] != float64(40) {
		t.Fatalf("payload = %v", payload)
	}
	if member.AccessLevel != 40 || member.Source != "direct" {
		t.Fatalf("member = %+v", member)
	}
}

func TestUpdateMemberRoleValidatesInput(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := client.UpdateMemberRole(context.Background(), "demo/app", 0, 40); err == nil {
		t.Fatal("expected an error for a missing user id")
	}
	if _, err := client.UpdateMemberRole(context.Background(), "demo/app", 8, 0); err == nil {
		t.Fatal("expected an error for a missing access level")
	}
}

func TestUpdateMemberExpiry(t *testing.T) {
	var method, escaped string
	var payload map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		escaped = r.URL.EscapedPath()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":8,"username":"demo","access_level":30,"expires_at":"2026-12-31"}`))
	})

	member, err := client.UpdateMemberExpiry(context.Background(), "demo/app", 8, 30, "2026-12-31")
	if err != nil {
		t.Fatalf("UpdateMemberExpiry: %v", err)
	}
	if method != http.MethodPut || escaped != "/api/v4/projects/demo%2Fapp/members/8" {
		t.Fatalf("method = %q, path = %q", method, escaped)
	}
	if payload["access_level"] != float64(30) {
		t.Fatalf("access_level = %v", payload["access_level"])
	}
	if payload["expires_at"] != "2026-12-31" {
		t.Fatalf("expires_at = %v", payload["expires_at"])
	}
	if member.ExpiresAt != "2026-12-31" {
		t.Fatalf("member = %+v", member)
	}
}

func TestUpdateMemberExpiryClearsDate(t *testing.T) {
	var payload map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&payload)
		_, _ = w.Write([]byte(`{"id":8,"username":"demo","access_level":30}`))
	})

	if _, err := client.UpdateMemberExpiry(context.Background(), "demo/app", 8, 30, ""); err != nil {
		t.Fatalf("UpdateMemberExpiry: %v", err)
	}
	value, ok := payload["expires_at"]
	if !ok || value != nil {
		t.Fatalf("expires_at should be null when cleared, payload = %v", payload)
	}
}

func TestUpdateMemberExpiryValidatesInput(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := client.UpdateMemberExpiry(context.Background(), "demo/app", 0, 30, "2026-12-31"); err == nil {
		t.Fatal("expected an error for a missing user id")
	}
	if _, err := client.UpdateMemberExpiry(context.Background(), "demo/app", 8, 0, "2026-12-31"); err == nil {
		t.Fatal("expected an error for a missing access level")
	}
}

func TestRemoveMember(t *testing.T) {
	var method, escaped string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		escaped = r.URL.EscapedPath()
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.RemoveMember(context.Background(), "demo/app", 8); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if method != http.MethodDelete {
		t.Fatalf("method = %q", method)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/members/8" {
		t.Fatalf("path = %q", escaped)
	}
}

func TestRemoveMemberValidatesInput(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if err := client.RemoveMember(context.Background(), "demo/app", 0); err == nil {
		t.Fatal("expected an error for a missing user id")
	}
}

func TestSearchUsers(t *testing.T) {
	var escaped string
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[{"id":8,"username":"user3","name":"测试用户","state":"active"}]`))
	})

	users, err := client.SearchUsers(context.Background(), " 测试 ", 20)
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if escaped != "/api/v4/users" {
		t.Fatalf("path = %q", escaped)
	}
	if query.Get("search") != "测试" {
		t.Fatalf("query = %v", query)
	}
	if len(users) != 1 || users[0].Username != "user3" {
		t.Fatalf("users = %+v", users)
	}
}

func TestSearchUsersRequiresQuery(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := client.SearchUsers(context.Background(), "   ", 20); err == nil {
		t.Fatal("expected an error for an empty query")
	}
}

func TestAddProjectMember(t *testing.T) {
	var method, escaped string
	var payload map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		escaped = r.URL.EscapedPath()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":8,"username":"user3","access_level":30,"expires_at":"2026-11-19"}`))
	})

	member, err := client.AddProjectMember(context.Background(), "demo/app", AddMemberOptions{
		Username:    " user3 ",
		AccessLevel: 30,
		ExpiresAt:   "2026-11-19",
	})
	if err != nil {
		t.Fatalf("AddProjectMember: %v", err)
	}
	if method != http.MethodPost || escaped != "/api/v4/projects/demo%2Fapp/members" {
		t.Fatalf("method = %q, path = %q", method, escaped)
	}
	if payload["username"] != "user3" || payload["access_level"] != float64(30) {
		t.Fatalf("payload = %v", payload)
	}
	if payload["expires_at"] != "2026-11-19" {
		t.Fatalf("expires_at = %v", payload["expires_at"])
	}
	if member.Source != "direct" || member.AccessLevel != 30 {
		t.Fatalf("member = %+v", member)
	}
}

func TestAddProjectMemberOmitsEmptyExpiry(t *testing.T) {
	var payload map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&payload)
		_, _ = w.Write([]byte(`{"id":8,"username":"user3","access_level":30}`))
	})

	if _, err := client.AddProjectMember(context.Background(), "demo/app", AddMemberOptions{
		Username:    "user3",
		AccessLevel: 30,
	}); err != nil {
		t.Fatalf("AddProjectMember: %v", err)
	}
	if _, ok := payload["expires_at"]; ok {
		t.Fatalf("expires_at should be omitted: %v", payload)
	}
}

func TestAddProjectMemberValidatesInput(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := client.AddProjectMember(context.Background(), "demo/app", AddMemberOptions{AccessLevel: 30}); err == nil {
		t.Fatal("expected an error for a missing username")
	}
	if _, err := client.AddProjectMember(context.Background(), "demo/app", AddMemberOptions{Username: "user3"}); err == nil {
		t.Fatal("expected an error for a missing access level")
	}
}

func TestInviteProjectMember(t *testing.T) {
	var method, escaped string
	var payload map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		escaped = r.URL.EscapedPath()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":9,"username":"invitee","access_level":10,"invite_email":"invitee@example.com"}`))
	})

	member, err := client.InviteProjectMember(context.Background(), "demo/app", InviteMemberOptions{
		Email:       "invitee@example.com",
		AccessLevel: 10,
		ExpiresAt:   "2026-12-31",
	})
	if err != nil {
		t.Fatalf("InviteProjectMember: %v", err)
	}
	if method != http.MethodPost || escaped != "/api/v4/projects/demo%2Fapp/invitations" {
		t.Fatalf("method = %q, path = %q", method, escaped)
	}
	if payload["email"] != "invitee@example.com" || payload["access_level"] != float64(10) {
		t.Fatalf("payload = %v", payload)
	}
	if payload["expires_at"] != "2026-12-31" {
		t.Fatalf("expires_at = %v", payload["expires_at"])
	}
	if member.Source != "direct" || member.Username != "invitee" {
		t.Fatalf("member = %+v", member)
	}
}

func TestInviteProjectMemberValidatesInput(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := client.InviteProjectMember(context.Background(), "demo/app", InviteMemberOptions{AccessLevel: 10}); err == nil {
		t.Fatal("expected an error for a missing email")
	}
	if _, err := client.InviteProjectMember(context.Background(), "demo/app", InviteMemberOptions{Email: "user@example.com"}); err == nil {
		t.Fatal("expected an error for a missing access level")
	}
}

func TestSearchBlobs(t *testing.T) {
	var escaped string
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[{"basename":"order.go","path":"core/order.go","data":"sampleSymbol","startline":42}]`))
	})

	page, err := client.SearchBlobs(context.Background(), "demo/app", "sampleSymbol", 1, 20)
	if err != nil {
		t.Fatalf("SearchBlobs: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/search" {
		t.Fatalf("path = %q", escaped)
	}
	if query.Get("scope") != "blobs" || query.Get("search") != "sampleSymbol" {
		t.Fatalf("query = %v", query)
	}
	if len(page.Items) != 1 || page.Items[0].StartLine != 42 {
		t.Fatalf("items = %+v", page.Items)
	}
}

func TestSearchBlobsRequiresQuery(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := client.SearchBlobs(context.Background(), "demo/app", "  ", 1, 20); err == nil {
		t.Fatal("expected an error for an empty query")
	}
}

func TestPipelineTestReport(t *testing.T) {
	var escaped string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{
			"total_count":3,"success_count":2,"failed_count":1,"skipped_count":0,"error_count":0,"total_time":1.5,
			"test_suites":[{"name":"core","total_count":3,"failed_count":1,
				"test_cases":[{"name":"TestDualTrailer","status":"failed","file":"dual_test.go","execution_time":0.4}]}]
		}`))
	})

	report, err := client.PipelineTestReport(context.Background(), "demo/app", 55)
	if err != nil {
		t.Fatalf("PipelineTestReport: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/pipelines/55/test_report" {
		t.Fatalf("path = %q", escaped)
	}
	if report.TotalCount != 3 || report.FailedCount != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.TestSuites) != 1 || len(report.TestSuites[0].TestCases) != 1 {
		t.Fatalf("suites = %+v", report.TestSuites)
	}
}

func TestJobArtifacts(t *testing.T) {
	var escaped string
	payload := []byte("PK\x03\x04fake-zip")
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		w.Header().Set("Content-Disposition", `attachment; filename="artifacts.zip"`)
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(payload)
	})

	artifact, err := client.JobArtifacts(context.Background(), "demo/app", 901, 1024)
	if err != nil {
		t.Fatalf("JobArtifacts: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/jobs/901/artifacts" {
		t.Fatalf("path = %q", escaped)
	}
	if artifact.Filename != "artifacts.zip" || artifact.Encoding != "base64" {
		t.Fatalf("artifact = %+v", artifact)
	}
	decoded, err := base64.StdEncoding.DecodeString(artifact.Data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(decoded) != string(payload) {
		t.Fatalf("payload mismatch: %q", decoded)
	}
}

func TestJobArtifactsTooLarge(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("0123456789"))
	})

	artifact, err := client.JobArtifacts(context.Background(), "demo/app", 901, 4)
	if err != nil {
		t.Fatalf("JobArtifacts: %v", err)
	}
	if !artifact.TooLarge || artifact.Data != "" {
		t.Fatalf("artifact = %+v", artifact)
	}
	if artifact.Size != 10 {
		t.Fatalf("size = %d", artifact.Size)
	}
}
