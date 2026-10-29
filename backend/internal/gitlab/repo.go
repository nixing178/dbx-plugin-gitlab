package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// Tree 列出仓库目录节点。path 为空表示仓库根目录。
func (c *Client) Tree(
	ctx context.Context,
	projectID, path, ref string,
	page, perPage int,
) (*TreePage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	query := pagingQuery(page, perPage)
	if trimmed := strings.Trim(strings.TrimSpace(path), "/"); trimmed != "" {
		query.Set("path", trimmed)
	}
	if trimmed := strings.TrimSpace(ref); trimmed != "" {
		query.Set("ref", trimmed)
	}

	var items []TreeNode
	header, err := c.getJSON(ctx, base+"/repository/tree", query, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &TreePage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}

// File 读取文件原文，超过上限时只返回元信息。
func (c *Client) File(ctx context.Context, projectID, filePath, ref string) (*FileContent, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	trimmedPath := strings.Trim(strings.TrimSpace(filePath), "/")
	if trimmedPath == "" {
		return nil, invalidConfig("file path is required")
	}

	query := url.Values{}
	if trimmedRef := strings.TrimSpace(ref); trimmedRef != "" {
		query.Set("ref", trimmedRef)
	}

	response, err := c.do(
		ctx,
		http.MethodGet,
		base+"/repository/files/"+url.PathEscape(trimmedPath)+"/raw",
		query,
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return c.readFileBody(response, trimmedPath, ref)
}

// Commits 列出提交历史。
func (c *Client) Commits(
	ctx context.Context,
	projectID, ref string,
	page, perPage int,
) (*CommitPage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	query := pagingQuery(page, perPage)
	if trimmed := strings.TrimSpace(ref); trimmed != "" {
		query.Set("ref_name", trimmed)
	}

	var items []Commit
	header, err := c.getJSON(ctx, base+"/repository/commits", query, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, total := pageFromHeader(header)
	return &CommitPage{Items: ensureSlice(items), Page: current, HasMore: hasMore, Total: total}, nil
}

// CommitDiff 读取单次提交的差异。
func (c *Client) CommitDiff(ctx context.Context, projectID, sha string) ([]DiffFile, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	trimmedSHA := strings.TrimSpace(sha)
	if trimmedSHA == "" {
		return nil, invalidConfig("commit sha is required")
	}

	var files []DiffFile
	if _, err := c.getJSON(
		ctx,
		base+"/repository/commits/"+url.PathEscape(trimmedSHA)+"/diff",
		nil,
		&files,
	); err != nil {
		return nil, err
	}
	return ensureSlice(files), nil
}
