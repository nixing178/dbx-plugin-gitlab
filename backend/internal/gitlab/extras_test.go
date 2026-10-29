package gitlab

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"
)

func TestProjectMembers(t *testing.T) {
	var escaped string
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[{"id":7,"username":"demo","name":"Demo User","access_level":40,"state":"active"}]`))
	})

	page, err := client.ProjectMembers(context.Background(), "demo/app", "sample", 1, 50)
	if err != nil {
		t.Fatalf("ProjectMembers: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/members/all" {
		t.Fatalf("path = %q", escaped)
	}
	if query.Get("query") != "sample" {
		t.Fatalf("query = %v", query)
	}
	if len(page.Items) != 1 || page.Items[0].AccessLevel != 40 {
		t.Fatalf("items = %+v", page.Items)
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
