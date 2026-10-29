package gitlab

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

// IssueOptions 是 Issue 列表的筛选条件。
type IssueOptions struct {
	ProjectID string
	// Scope 仅在全局视图生效：assigned_to_me / created_by_me / all。
	Scope   string
	State   string
	Search  string
	Labels  string
	Page    int
	PerPage int
}

// Issues 列出 Issue。ProjectID 为空时走全局视角。
func (c *Client) Issues(ctx context.Context, opts IssueOptions) (*IssuePage, error) {
	query := pagingQuery(opts.Page, opts.PerPage)
	if state := strings.TrimSpace(opts.State); state != "" {
		query.Set("state", state)
	}
	if search := strings.TrimSpace(opts.Search); search != "" {
		query.Set("search", search)
	}
	if labels := strings.TrimSpace(opts.Labels); labels != "" {
		query.Set("labels", labels)
	}

	path := "/issues"
	if strings.TrimSpace(opts.ProjectID) != "" {
		base, err := projectPath(opts.ProjectID)
		if err != nil {
			return nil, err
		}
		path = base + "/issues"
	} else if scope := strings.TrimSpace(opts.Scope); scope != "" {
		query.Set("scope", scope)
	}

	var items []Issue
	header, err := c.getJSON(ctx, path, query, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &IssuePage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}

// Issue 读取单个 Issue。
func (c *Client) Issue(ctx context.Context, projectID string, iid int) (*Issue, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("issue iid is required")
	}

	var issue Issue
	if _, err := c.getJSON(ctx, base+"/issues/"+strconv.Itoa(iid), nil, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// IssueNotes 读取 Issue 的评论。
func (c *Client) IssueNotes(ctx context.Context, projectID string, iid int) ([]Note, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("issue iid is required")
	}

	var items []Note
	if _, err := c.getJSON(ctx, base+"/issues/"+strconv.Itoa(iid)+"/notes", nil, &items); err != nil {
		return nil, err
	}
	return ensureSlice(items), nil
}

// CreateIssueInput 是新建 Issue 的字段。
type CreateIssueInput struct {
	Title       string
	Description string
	Labels      []string
	AssigneeIDs []int
	DueDate     string
}

// CreateIssue 新建 Issue。
func (c *Client) CreateIssue(
	ctx context.Context,
	projectID string,
	input CreateIssueInput,
) (*Issue, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, invalidConfig("issue title is required")
	}

	payload := map[string]any{"title": title}
	if description := strings.TrimSpace(input.Description); description != "" {
		payload["description"] = description
	}
	if len(input.Labels) > 0 {
		payload["labels"] = strings.Join(input.Labels, ",")
	}
	if len(input.AssigneeIDs) > 0 {
		payload["assignee_ids"] = input.AssigneeIDs
	}
	if due := strings.TrimSpace(input.DueDate); due != "" {
		payload["due_date"] = due
	}

	var issue Issue
	if _, err := c.writeJSON(ctx, http.MethodPost, base+"/issues", nil, payload, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// UpdateIssueInput 是编辑 Issue 的字段；nil 表示不修改。
type UpdateIssueInput struct {
	Title       *string
	Description *string
	StateEvent  string
	Labels      []string
	AssigneeIDs []int
}

// UpdateIssue 编辑 Issue 或改变状态。
func (c *Client) UpdateIssue(
	ctx context.Context,
	projectID string,
	iid int,
	input UpdateIssueInput,
) (*Issue, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("issue iid is required")
	}

	payload := map[string]any{}
	if input.Title != nil {
		payload["title"] = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		payload["description"] = *input.Description
	}
	if event := strings.TrimSpace(input.StateEvent); event != "" {
		if event != "close" && event != "reopen" {
			return nil, invalidConfig("state event must be close or reopen")
		}
		payload["state_event"] = event
	}
	if input.Labels != nil {
		payload["labels"] = strings.Join(input.Labels, ",")
	}
	if input.AssigneeIDs != nil {
		payload["assignee_ids"] = input.AssigneeIDs
	}
	if len(payload) == 0 {
		return nil, invalidConfig("no fields to update")
	}

	var issue Issue
	if _, err := c.writeJSON(
		ctx,
		http.MethodPut,
		base+"/issues/"+strconv.Itoa(iid),
		nil,
		payload,
		&issue,
	); err != nil {
		return nil, err
	}
	return &issue, nil
}

// CommentIssue 在 Issue 下发评论。
func (c *Client) CommentIssue(
	ctx context.Context,
	projectID string,
	iid int,
	body string,
) (*Note, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("issue iid is required")
	}
	text := strings.TrimSpace(body)
	if text == "" {
		return nil, invalidConfig("comment body is required")
	}

	var note Note
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/issues/"+strconv.Itoa(iid)+"/notes",
		nil,
		map[string]any{"body": text},
		&note,
	); err != nil {
		return nil, err
	}
	return &note, nil
}

// Todos 列出当前用户的待办。
func (c *Client) Todos(ctx context.Context, state string, page, perPage int) (*TodoPage, error) {
	query := pagingQuery(page, perPage)
	if trimmed := strings.TrimSpace(state); trimmed != "" {
		query.Set("state", trimmed)
	}

	var items []Todo
	header, err := c.getJSON(ctx, "/todos", query, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &TodoPage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}

// MarkTodoDone 把待办标记为已完成。
func (c *Client) MarkTodoDone(ctx context.Context, todoID int) (*Todo, error) {
	if todoID <= 0 {
		return nil, invalidConfig("todo id is required")
	}

	var todo Todo
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		"/todos/"+strconv.Itoa(todoID)+"/mark_as_done",
		nil,
		nil,
		&todo,
	); err != nil {
		return nil, err
	}
	return &todo, nil
}
