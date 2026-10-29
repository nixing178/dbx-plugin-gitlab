package gitlab

import (
	"context"
	"strings"
)

// Label 是项目标签。
type Label struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	TextColor   string `json:"text_color"`
	Description string `json:"description"`
}

// LabelPage 是标签分页结果。
type LabelPage struct {
	Items   []Label `json:"items"`
	Page    int     `json:"page"`
	HasMore bool    `json:"has_more"`
}

// MilestonePage 是里程碑分页结果。
type MilestonePage struct {
	Items   []Milestone `json:"items"`
	Page    int         `json:"page"`
	HasMore bool        `json:"has_more"`
}

// Labels 列出项目标签，search 走服务端过滤。
func (c *Client) Labels(
	ctx context.Context,
	projectID, search string,
	page, perPage int,
) (*LabelPage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	query := pagingQuery(page, perPage)
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		query.Set("search", trimmed)
	}

	var items []Label
	header, err := c.getJSON(ctx, base+"/labels", query, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &LabelPage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}

// Milestones 列出项目里程碑；state 为空表示不限状态。
func (c *Client) Milestones(
	ctx context.Context,
	projectID, state string,
	page, perPage int,
) (*MilestonePage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	query := pagingQuery(page, perPage)
	if trimmed := strings.TrimSpace(state); trimmed != "" {
		query.Set("state", trimmed)
	}

	var items []Milestone
	header, err := c.getJSON(ctx, base+"/milestones", query, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &MilestonePage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}
