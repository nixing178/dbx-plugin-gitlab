package gitlab

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// defaultMaxArtifactBytes 限制单次制品下载，避免把大 zip 全塞进内存。
const defaultMaxArtifactBytes = 8 << 20

// ProjectMembers 列出项目成员（包含从上层组继承的成员）。
func (c *Client) ProjectMembers(
	ctx context.Context,
	projectID, query string,
	page, perPage int,
) (*MemberPage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	params := pagingQuery(page, perPage)
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		params.Set("query", trimmed)
	}

	var items []Member
	header, err := c.getJSON(ctx, base+"/members/all", params, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &MemberPage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}

// SearchBlobs 在项目默认分支内搜索代码。
func (c *Client) SearchBlobs(
	ctx context.Context,
	projectID, search string,
	page, perPage int,
) (*SearchPage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(search)
	if trimmed == "" {
		return nil, invalidConfig("search query is required")
	}

	params := pagingQuery(page, perPage)
	params.Set("scope", "blobs")
	params.Set("search", trimmed)

	var items []SearchResult
	header, err := c.getJSON(ctx, base+"/search", params, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &SearchPage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}

// PipelineTestReport 读取流水线的测试报告。
func (c *Client) PipelineTestReport(
	ctx context.Context,
	projectID string,
	pipelineID int,
) (*TestReport, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if pipelineID <= 0 {
		return nil, invalidConfig("pipeline id is required")
	}

	var report TestReport
	if _, err := c.getJSON(
		ctx,
		base+"/pipelines/"+strconv.Itoa(pipelineID)+"/test_report",
		nil,
		&report,
	); err != nil {
		return nil, err
	}
	return &report, nil
}

// JobArtifacts 下载作业制品；超过 maxBytes 时只返回大小。
func (c *Client) JobArtifacts(
	ctx context.Context,
	projectID string,
	jobID int,
	maxBytes int64,
) (*ArtifactDownload, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if jobID <= 0 {
		return nil, invalidConfig("job id is required")
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxArtifactBytes
	}

	if err := c.ensureAuthorized(ctx); err != nil {
		return nil, err
	}

	endpoint := c.baseURL + "/api/v4" + base + "/jobs/" + strconv.Itoa(jobID) + "/artifacts"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, invalidConfig("cannot build request: %v", err)
	}
	c.applyAuth(request)
	request.Header.Set("User-Agent", userAgent)

	response, err := c.http.Do(request)
	if err != nil {
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
		return nil, mapHTTPError(response, payload)
	}

	filename := parseContentDispositionFilename(response.Header.Get("Content-Disposition"))

	size, _ := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if size > maxBytes {
		return &ArtifactDownload{Filename: filename, Size: size, TooLarge: true}, nil
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	if int64(len(payload)) > maxBytes {
		return &ArtifactDownload{
			Filename: filename,
			Size:     int64(len(payload)),
			TooLarge: true,
		}, nil
	}

	if filename == "" {
		filename = "job-" + strconv.Itoa(jobID) + "-artifacts.zip"
	}

	return &ArtifactDownload{
		Filename: filename,
		Size:     int64(len(payload)),
		Encoding: "base64",
		Data:     base64.StdEncoding.EncodeToString(payload),
	}, nil
}

// parseContentDispositionFilename 从 Content-Disposition 里取文件名。
func parseContentDispositionFilename(value string) string {
	for _, segment := range strings.Split(value, ";") {
		trimmed := strings.TrimSpace(segment)
		if !strings.HasPrefix(trimmed, "filename=") {
			continue
		}
		name := strings.TrimPrefix(trimmed, "filename=")
		return strings.Trim(name, `"`)
	}
	return ""
}
