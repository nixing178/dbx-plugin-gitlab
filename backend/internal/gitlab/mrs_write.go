package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// PositionInput 是行内评论的定位参数。
type PositionInput struct {
	BaseSHA      string `json:"base_sha"`
	HeadSHA      string `json:"head_sha"`
	StartSHA     string `json:"start_sha"`
	NewPath      string `json:"new_path"`
	OldPath      string `json:"old_path"`
	PositionType string `json:"position_type"`
	NewLine      int    `json:"new_line,omitempty"`
	OldLine      int    `json:"old_line,omitempty"`
}

// MergeOptions 是合并合并请求的选项。
type MergeOptions struct {
	SHA                 string
	Squash              bool
	RemoveSourceBranch  bool
	MergeCommitMessage  string
	SquashCommitMessage string
}

// CreateMRInput 是创建合并请求的字段。
type CreateMRInput struct {
	SourceBranch       string
	TargetBranch       string
	Title              string
	Description        string
	AssigneeIDs        []int
	ReviewerIDs        []int
	Labels             []string
	MilestoneID        int
	RemoveSourceBranch bool
	Squash             bool
}

// UpdateMRInput 是编辑合并请求的字段；nil 表示不修改。
// MilestoneID 指向 0 时表示清空里程碑（GitLab 的 milestone_id=0 语义）。
type UpdateMRInput struct {
	Title        *string
	Description  *string
	TargetBranch *string
	AssigneeIDs  []int
	ReviewerIDs  []int
	Labels       []string
	MilestoneID  *int
}

// CommentMR 新建一条讨论；position 非空时是行内评论。
func (c *Client) CommentMR(
	ctx context.Context,
	projectID string,
	iid int,
	body string,
	position *PositionInput,
) (*Note, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}
	text := strings.TrimSpace(body)
	if text == "" {
		return nil, invalidConfig("comment body is required")
	}

	payload := map[string]any{"body": text}
	if position != nil {
		if strings.TrimSpace(position.NewPath) == "" {
			return nil, invalidConfig("position path is required for an inline comment")
		}
		payload["position"] = position
	}

	var discussion Discussion
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/merge_requests/"+strconv.Itoa(iid)+"/discussions",
		nil,
		payload,
		&discussion,
	); err != nil {
		return nil, err
	}
	if len(discussion.Notes) == 0 {
		return nil, &APIError{Kind: ErrServer, Message: "GitLab returned no note for the new discussion"}
	}
	return &discussion.Notes[0], nil
}

// ReplyDiscussion 在已有讨论线程里回复。
func (c *Client) ReplyDiscussion(
	ctx context.Context,
	projectID string,
	iid int,
	discussionID, body string,
) (*Note, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}
	thread := strings.TrimSpace(discussionID)
	if thread == "" {
		return nil, invalidConfig("discussion id is required")
	}
	text := strings.TrimSpace(body)
	if text == "" {
		return nil, invalidConfig("comment body is required")
	}

	var note Note
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/merge_requests/"+strconv.Itoa(iid)+"/discussions/"+url.PathEscape(thread)+"/notes",
		nil,
		map[string]any{"body": text},
		&note,
	); err != nil {
		return nil, err
	}
	return &note, nil
}

// ResolveDiscussion 标记讨论已解决/未解决。
func (c *Client) ResolveDiscussion(
	ctx context.Context,
	projectID string,
	iid int,
	discussionID string,
	resolved bool,
) (*Discussion, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}
	thread := strings.TrimSpace(discussionID)
	if thread == "" {
		return nil, invalidConfig("discussion id is required")
	}

	query := url.Values{}
	query.Set("resolved", strconv.FormatBool(resolved))

	var discussion Discussion
	if _, err := c.writeJSON(
		ctx,
		http.MethodPut,
		base+"/merge_requests/"+strconv.Itoa(iid)+"/discussions/"+url.PathEscape(thread),
		query,
		nil,
		&discussion,
	); err != nil {
		return nil, err
	}
	return &discussion, nil
}

// ApproveMR 通过合并请求；sha 非空时锁定到指定提交。
func (c *Client) ApproveMR(ctx context.Context, projectID string, iid int, sha string) (*Approval, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}

	payload := map[string]any{}
	if trimmed := strings.TrimSpace(sha); trimmed != "" {
		payload["sha"] = trimmed
	}

	var approval Approval
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/merge_requests/"+strconv.Itoa(iid)+"/approve",
		nil,
		payload,
		&approval,
	); err != nil {
		return nil, err
	}
	return &approval, nil
}

// UnapproveMR 撤销审批。
func (c *Client) UnapproveMR(ctx context.Context, projectID string, iid int) error {
	base, err := projectPath(projectID)
	if err != nil {
		return err
	}
	if iid <= 0 {
		return invalidConfig("merge request iid is required")
	}

	_, err = c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/merge_requests/"+strconv.Itoa(iid)+"/unapprove",
		nil,
		nil,
		nil,
	)
	return err
}

// MergeMR 合并合并请求。
func (c *Client) MergeMR(
	ctx context.Context,
	projectID string,
	iid int,
	opts MergeOptions,
) (*MR, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}

	payload := map[string]any{
		"squash":                      opts.Squash,
		"should_remove_source_branch": opts.RemoveSourceBranch,
	}
	if trimmed := strings.TrimSpace(opts.SHA); trimmed != "" {
		payload["sha"] = trimmed
	}
	if trimmed := strings.TrimSpace(opts.MergeCommitMessage); trimmed != "" {
		payload["merge_commit_message"] = trimmed
	}
	if trimmed := strings.TrimSpace(opts.SquashCommitMessage); trimmed != "" {
		payload["squash_commit_message"] = trimmed
	}

	var mr MR
	if _, err := c.writeJSON(
		ctx,
		http.MethodPut,
		base+"/merge_requests/"+strconv.Itoa(iid)+"/merge",
		nil,
		payload,
		&mr,
	); err != nil {
		return nil, err
	}
	return &mr, nil
}

// SetMRState 关闭或重开合并请求。
func (c *Client) SetMRState(ctx context.Context, projectID string, iid int, event string) (*MR, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}
	stateEvent := strings.TrimSpace(event)
	if stateEvent != "close" && stateEvent != "reopen" {
		return nil, invalidConfig("state event must be close or reopen")
	}

	var mr MR
	if _, err := c.writeJSON(
		ctx,
		http.MethodPut,
		base+"/merge_requests/"+strconv.Itoa(iid),
		nil,
		map[string]any{"state_event": stateEvent},
		&mr,
	); err != nil {
		return nil, err
	}
	return &mr, nil
}

// CreateMR 创建合并请求。
func (c *Client) CreateMR(ctx context.Context, projectID string, input CreateMRInput) (*MR, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	source := strings.TrimSpace(input.SourceBranch)
	target := strings.TrimSpace(input.TargetBranch)
	title := strings.TrimSpace(input.Title)
	if source == "" || target == "" {
		return nil, invalidConfig("source and target branches are required")
	}
	if source == target {
		return nil, invalidConfig("source and target branches must differ")
	}
	if title == "" {
		return nil, invalidConfig("title is required")
	}

	payload := map[string]any{
		"source_branch":        source,
		"target_branch":        target,
		"title":                title,
		"remove_source_branch": input.RemoveSourceBranch,
		"squash":               input.Squash,
	}
	if description := strings.TrimSpace(input.Description); description != "" {
		payload["description"] = description
	}
	if len(input.AssigneeIDs) > 0 {
		payload["assignee_ids"] = input.AssigneeIDs
	}
	if len(input.ReviewerIDs) > 0 {
		payload["reviewer_ids"] = input.ReviewerIDs
	}
	if len(input.Labels) > 0 {
		payload["labels"] = strings.Join(input.Labels, ",")
	}
	if input.MilestoneID > 0 {
		payload["milestone_id"] = input.MilestoneID
	}

	var mr MR
	if _, err := c.writeJSON(ctx, http.MethodPost, base+"/merge_requests", nil, payload, &mr); err != nil {
		return nil, err
	}
	return &mr, nil
}

// UpdateMR 编辑合并请求，只提交显式设置的字段。
func (c *Client) UpdateMR(
	ctx context.Context,
	projectID string,
	iid int,
	input UpdateMRInput,
) (*MR, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}

	payload := map[string]any{}
	if input.Title != nil {
		payload["title"] = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		payload["description"] = *input.Description
	}
	if input.TargetBranch != nil {
		payload["target_branch"] = strings.TrimSpace(*input.TargetBranch)
	}
	if input.AssigneeIDs != nil {
		payload["assignee_ids"] = input.AssigneeIDs
	}
	if input.ReviewerIDs != nil {
		payload["reviewer_ids"] = input.ReviewerIDs
	}
	if input.Labels != nil {
		payload["labels"] = strings.Join(input.Labels, ",")
	}
	if input.MilestoneID != nil {
		payload["milestone_id"] = *input.MilestoneID
	}
	if len(payload) == 0 {
		return nil, invalidConfig("no fields to update")
	}

	var mr MR
	if _, err := c.writeJSON(
		ctx,
		http.MethodPut,
		base+"/merge_requests/"+strconv.Itoa(iid),
		nil,
		payload,
		&mr,
	); err != nil {
		return nil, err
	}
	return &mr, nil
}

// MRVersions 列出合并请求的变更版本（force-push 会产生新版本）。
func (c *Client) MRVersions(ctx context.Context, projectID string, iid int) ([]MRVersion, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}

	var items []MRVersion
	if _, err := c.getJSON(
		ctx,
		base+"/merge_requests/"+strconv.Itoa(iid)+"/versions",
		nil,
		&items,
	); err != nil {
		return nil, err
	}
	return ensureSlice(items), nil
}

// DecodePosition 把 MCP 参数里的 position 对象转换为结构体。
func DecodePosition(value any) (*PositionInput, error) {
	if value == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, invalidConfig("invalid position payload")
	}
	var position PositionInput
	if err := json.Unmarshal(encoded, &position); err != nil {
		return nil, invalidConfig("invalid position payload")
	}
	return &position, nil
}
