package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// graphqlJSON 直接请求 /api/graphql（GraphQL 不在 /api/v4 前缀下）。
func (c *Client) graphqlJSON(ctx context.Context, query string, variables map[string]any, out any) error {
	if err := c.ensureAuthorized(ctx); err != nil {
		return err
	}

	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return invalidConfig("cannot encode GraphQL request")
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/graphql",
		bytes.NewReader(body),
	)
	if err != nil {
		return invalidConfig("cannot build GraphQL request: %v", err)
	}
	c.applyAuth(request)
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, maxJSONBody))
	if err != nil {
		return &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return mapHTTPError(response, payload)
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return &APIError{Kind: ErrServer, Message: "unexpected GraphQL response format"}
	}
	return nil
}

type viewerStarResponse struct {
	Data struct {
		Project *struct {
			ViewerHasStarred bool `json:"viewerHasStarred"`
		} `json:"project"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// viewerHasStarred 用 GraphQL 精确查询当前用户是否已加星（REST 没有该字段）。
// ok=false 表示实例不支持或查询失败，调用方退回 starred 列表反查。
func (c *Client) viewerHasStarred(ctx context.Context, fullPath string) (starred bool, ok bool) {
	path := strings.TrimSpace(fullPath)
	if path == "" {
		return false, false
	}

	var result viewerStarResponse
	err := c.graphqlJSON(
		ctx,
		`query($path: ID!) { project(fullPath: $path) { viewerHasStarred } }`,
		map[string]any{"path": path},
		&result,
	)
	if err != nil || len(result.Errors) > 0 || result.Data.Project == nil {
		return false, false
	}
	return result.Data.Project.ViewerHasStarred, true
}
