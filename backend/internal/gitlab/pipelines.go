package gitlab

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// maxTraceBytes 限制单次日志增量，避免一次拉回超大日志。
const maxTraceBytes = 2 << 20

// PipelineOptions 是流水线列表的筛选条件。
type PipelineOptions struct {
	Ref     string
	Status  string
	Page    int
	PerPage int
}

// Pipelines 列出项目的流水线。
func (c *Client) Pipelines(
	ctx context.Context,
	projectID string,
	opts PipelineOptions,
) (*PipelinePage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	query := pagingQuery(opts.Page, opts.PerPage)
	if ref := strings.TrimSpace(opts.Ref); ref != "" {
		query.Set("ref", ref)
	}
	if status := strings.TrimSpace(opts.Status); status != "" {
		query.Set("status", status)
	}

	var items []Pipeline
	header, err := c.getJSON(ctx, base+"/pipelines", query, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &PipelinePage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}

// Pipeline 读取流水线详情，包含阶段列表。
func (c *Client) Pipeline(ctx context.Context, projectID string, pipelineID int) (*PipelineDetail, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if pipelineID <= 0 {
		return nil, invalidConfig("pipeline id is required")
	}

	var detail PipelineDetail
	if _, err := c.getJSON(
		ctx,
		base+"/pipelines/"+strconv.Itoa(pipelineID),
		nil,
		&detail,
	); err != nil {
		return nil, err
	}
	return &detail, nil
}

// PipelineJobs 列出流水线作业，包含已重试的历史作业。
func (c *Client) PipelineJobs(
	ctx context.Context,
	projectID string,
	pipelineID, page, perPage int,
) (*JobPage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if pipelineID <= 0 {
		return nil, invalidConfig("pipeline id is required")
	}

	query := pagingQuery(page, perPage)
	query.Set("include_retried", "true")

	var items []Job
	header, err := c.getJSON(
		ctx,
		base+"/pipelines/"+strconv.Itoa(pipelineID)+"/jobs",
		query,
		&items,
	)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &JobPage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}

// JobTrace 增量读取作业日志。offset 为已读取的字节数。
func (c *Client) JobTrace(
	ctx context.Context,
	projectID string,
	jobID int,
	offset int64,
) (*JobTrace, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if jobID <= 0 {
		return nil, invalidConfig("job id is required")
	}
	if offset < 0 {
		offset = 0
	}

	if err := c.ensureAuthorized(ctx); err != nil {
		return nil, err
	}

	endpoint := c.baseURL + "/api/v4" + base + "/jobs/" + strconv.Itoa(jobID) + "/trace"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, invalidConfig("cannot build request: %v", err)
	}
	c.applyAuth(request)
	request.Header.Set("User-Agent", userAgent)
	if offset > 0 {
		request.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}

	response, err := c.http.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, &APIError{Kind: ErrNetwork, Message: ctxErr.Error()}
		}
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
		return nil, mapHTTPError(response, payload)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxTraceBytes))
	if err != nil {
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}

	size := parseContentRangeTotal(response.Header.Get("Content-Range"))
	if size == 0 {
		if length, convErr := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64); convErr == nil && length > 0 {
			size = offset + length
		}
	}

	next := offset + int64(len(body))
	if size > 0 && next > size {
		next = size
	}
	complete := size > 0 && next >= size

	return &JobTrace{
		Text:       string(body),
		NextOffset: next,
		Complete:   complete,
		Size:       size,
	}, nil
}

// parseContentRangeTotal 解析 "bytes 10-19/25" 里的总长度。
func parseContentRangeTotal(value string) int64 {
	index := strings.LastIndex(value, "/")
	if index < 0 {
		return 0
	}
	total, err := strconv.ParseInt(strings.TrimSpace(value[index+1:]), 10, 64)
	if err != nil || total < 0 {
		return 0
	}
	return total
}

func (c *Client) jobAction(
	ctx context.Context,
	projectID string,
	jobID int,
	action string,
) (*Job, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if jobID <= 0 {
		return nil, invalidConfig("job id is required")
	}

	var job Job
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/jobs/"+strconv.Itoa(jobID)+"/"+action,
		nil,
		nil,
		&job,
	); err != nil {
		return nil, err
	}
	return &job, nil
}

// RetryJob 重试作业。
func (c *Client) RetryJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	return c.jobAction(ctx, projectID, jobID, "retry")
}

// CancelJob 取消作业。
func (c *Client) CancelJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	return c.jobAction(ctx, projectID, jobID, "cancel")
}

// PlayJob 执行手动作业。
func (c *Client) PlayJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	return c.jobAction(ctx, projectID, jobID, "play")
}

func (c *Client) pipelineAction(
	ctx context.Context,
	projectID string,
	pipelineID int,
	action string,
) (*Pipeline, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if pipelineID <= 0 {
		return nil, invalidConfig("pipeline id is required")
	}

	var pipeline Pipeline
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/pipelines/"+strconv.Itoa(pipelineID)+"/"+action,
		nil,
		nil,
		&pipeline,
	); err != nil {
		return nil, err
	}
	return &pipeline, nil
}

// RetryPipeline 重试失败作业。
func (c *Client) RetryPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	return c.pipelineAction(ctx, projectID, pipelineID, "retry")
}

// CancelPipeline 取消流水线。
func (c *Client) CancelPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	return c.pipelineAction(ctx, projectID, pipelineID, "cancel")
}

// CreatePipeline 触发一条新流水线。
func (c *Client) CreatePipeline(
	ctx context.Context,
	projectID, ref string,
	variables []PipelineVariable,
) (*Pipeline, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	trimmedRef := strings.TrimSpace(ref)
	if trimmedRef == "" {
		return nil, invalidConfig("ref is required")
	}

	query := url.Values{}
	query.Set("ref", trimmedRef)

	payload := map[string]any{}
	if len(variables) > 0 {
		items := make([]map[string]string, 0, len(variables))
		for _, variable := range variables {
			key := strings.TrimSpace(variable.Key)
			if key == "" {
				continue
			}
			items = append(items, map[string]string{"key": key, "value": variable.Value})
		}
		if len(items) > 0 {
			payload["variables"] = items
		}
	}

	var pipeline Pipeline
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/pipeline",
		query,
		payload,
		&pipeline,
	); err != nil {
		return nil, err
	}
	return &pipeline, nil
}
