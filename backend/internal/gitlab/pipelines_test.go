package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

func TestPipelinesQuery(t *testing.T) {
	var query url.Values
	var escaped string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		escaped = r.URL.EscapedPath()
		w.Header().Set("X-Next-Page", "2")
		_, _ = w.Write([]byte(`[{"id":55,"status":"success","ref":"main","sha":"abc"}]`))
	})

	page, err := client.Pipelines(context.Background(), "demo/app", PipelineOptions{
		Ref:     "main",
		Status:  "success",
		Page:    1,
		PerPage: 20,
	})
	if err != nil {
		t.Fatalf("Pipelines: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/pipelines" {
		t.Fatalf("path = %q", escaped)
	}
	if query.Get("ref") != "main" || query.Get("status") != "success" {
		t.Fatalf("query = %v", query)
	}
	if query.Get("page") != "1" || query.Get("per_page") != "20" {
		t.Fatalf("paging = %v", query)
	}
	if len(page.Items) != 1 || page.Items[0].ID != 55 {
		t.Fatalf("items = %+v", page.Items)
	}
	if !page.HasMore {
		t.Fatal("has_more should be true")
	}
}

func TestPipelineDetailParsesStages(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/v4/projects/demo%2Fapp/pipelines/55" {
			t.Errorf("path = %q", r.URL.EscapedPath())
		}
		_, _ = w.Write([]byte(`{
			"id":55,"status":"running","ref":"main","sha":"abc","duration":42,
			"stages":[{"name":"build","status":"success"},{"name":"test","status":"running"}],
			"detailed_status":{"icon":"status_running","text":"running","group":"running"}
		}`))
	})

	detail, err := client.Pipeline(context.Background(), "demo/app", 55)
	if err != nil {
		t.Fatalf("Pipeline: %v", err)
	}
	if detail.ID != 55 || detail.Status != "running" {
		t.Fatalf("detail = %+v", detail)
	}
	if len(detail.Stages) != 2 || detail.Stages[1].Name != "test" {
		t.Fatalf("stages = %+v", detail.Stages)
	}
	if detail.DetailedStatus == nil || detail.DetailedStatus.Group != "running" {
		t.Fatalf("detailed status = %+v", detail.DetailedStatus)
	}
}

func TestPipelineJobsIncludesRetried(t *testing.T) {
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		_, _ = w.Write([]byte(`[{"id":901,"name":"compile","stage":"build","status":"failed","allow_failure":false}]`))
	})

	page, err := client.PipelineJobs(context.Background(), "demo/app", 55, 1, 100)
	if err != nil {
		t.Fatalf("PipelineJobs: %v", err)
	}
	if query.Get("include_retried") != "true" {
		t.Fatalf("include_retried = %q", query.Get("include_retried"))
	}
	if len(page.Items) != 1 || page.Items[0].Stage != "build" {
		t.Fatalf("items = %+v", page.Items)
	}
}

func TestJobTraceRequestsRange(t *testing.T) {
	var rangeHeader string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		rangeHeader = r.Header.Get("Range")
		w.Header().Set("Content-Range", "bytes 10-19/25")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("0123456789"))
	})

	trace, err := client.JobTrace(context.Background(), "demo/app", 901, 10)
	if err != nil {
		t.Fatalf("JobTrace: %v", err)
	}
	if rangeHeader != "bytes=10-" {
		t.Fatalf("Range = %q", rangeHeader)
	}
	if trace.Text != "0123456789" {
		t.Fatalf("text = %q", trace.Text)
	}
	if trace.NextOffset != 20 {
		t.Fatalf("next_offset = %d", trace.NextOffset)
	}
	if trace.Complete {
		t.Fatal("trace must not be complete at 20/25")
	}
	if trace.Size != 25 {
		t.Fatalf("size = %d", trace.Size)
	}
}

func TestJobTraceCompletesAtEnd(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=20-" {
			t.Errorf("Range = %q", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Range", "bytes 20-24/25")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("abcde"))
	})

	trace, err := client.JobTrace(context.Background(), "demo/app", 901, 20)
	if err != nil {
		t.Fatalf("JobTrace: %v", err)
	}
	if trace.NextOffset != 25 || !trace.Complete {
		t.Fatalf("trace = %+v", trace)
	}
}

func TestJobTraceFirstChunkOmitsRange(t *testing.T) {
	var rangeHeader string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		rangeHeader = r.Header.Get("Range")
		_, _ = w.Write([]byte("hello"))
	})

	trace, err := client.JobTrace(context.Background(), "demo/app", 901, 0)
	if err != nil {
		t.Fatalf("JobTrace: %v", err)
	}
	if rangeHeader != "" {
		t.Fatalf("Range must be empty for the first chunk, got %q", rangeHeader)
	}
	if trace.NextOffset != 5 || !trace.Complete {
		t.Fatalf("trace = %+v", trace)
	}
}

func TestJobActionsUsePost(t *testing.T) {
	var calls []string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.EscapedPath())
		_, _ = w.Write([]byte(`{"id":901,"status":"pending","name":"compile","stage":"build"}`))
	})

	ctx := context.Background()
	if _, err := client.RetryJob(ctx, "demo/app", 901); err != nil {
		t.Fatalf("RetryJob: %v", err)
	}
	if _, err := client.CancelJob(ctx, "demo/app", 901); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	if _, err := client.PlayJob(ctx, "demo/app", 901); err != nil {
		t.Fatalf("PlayJob: %v", err)
	}

	expected := []string{
		"POST /api/v4/projects/demo%2Fapp/jobs/901/retry",
		"POST /api/v4/projects/demo%2Fapp/jobs/901/cancel",
		"POST /api/v4/projects/demo%2Fapp/jobs/901/play",
	}
	for index, want := range expected {
		if index >= len(calls) || calls[index] != want {
			t.Fatalf("calls = %v, want %v", calls, expected)
		}
	}
}

func TestPipelineActionsUsePost(t *testing.T) {
	var calls []string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.EscapedPath())
		_, _ = w.Write([]byte(`{"id":55,"status":"pending","ref":"main"}`))
	})

	ctx := context.Background()
	if _, err := client.RetryPipeline(ctx, "demo/app", 55); err != nil {
		t.Fatalf("RetryPipeline: %v", err)
	}
	if _, err := client.CancelPipeline(ctx, "demo/app", 55); err != nil {
		t.Fatalf("CancelPipeline: %v", err)
	}

	expected := []string{
		"POST /api/v4/projects/demo%2Fapp/pipelines/55/retry",
		"POST /api/v4/projects/demo%2Fapp/pipelines/55/cancel",
	}
	for index, want := range expected {
		if index >= len(calls) || calls[index] != want {
			t.Fatalf("calls = %v, want %v", calls, expected)
		}
	}
}

func TestCreatePipelineSendsRefAndVariables(t *testing.T) {
	var escaped string
	var query url.Values
	var payload map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		escaped = r.URL.EscapedPath()
		query = r.URL.Query()
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":56,"status":"created","ref":"main"}`))
	})

	pipeline, err := client.CreatePipeline(context.Background(), "demo/app", "main", []PipelineVariable{
		{Key: "DEPLOY_ENV", Value: "prod"},
	})
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if escaped != "/api/v4/projects/demo%2Fapp/pipeline" {
		t.Fatalf("path = %q", escaped)
	}
	if query.Get("ref") != "main" {
		t.Fatalf("ref = %q", query.Get("ref"))
	}
	variables, ok := payload["variables"].([]any)
	if !ok || len(variables) != 1 {
		t.Fatalf("payload = %v", payload)
	}
	first, _ := variables[0].(map[string]any)
	if first["key"] != "DEPLOY_ENV" || first["value"] != "prod" {
		t.Fatalf("variables = %v", variables)
	}
	if pipeline == nil || pipeline.ID != 56 {
		t.Fatalf("pipeline = %+v", pipeline)
	}
}

func TestCreatePipelineRejectsEmptyRef(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := client.CreatePipeline(context.Background(), "demo/app", "   ", nil); err == nil {
		t.Fatal("expected an error for empty ref")
	}
}
