package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ApprovalVoter 是审批人包装：GitLab 把用户塞在 user 字段里。
type ApprovalVoter struct {
	User *User `json:"user"`
}

// MRApprovals 是合并请求的审批状态。
// GitLab 免费版只返回 0 个必需审批，此时 approvals_required 为 0，UI 显示「核准为可选」。
type MRApprovals struct {
	ID                 int             `json:"id"`
	IID                int             `json:"iid"`
	ApprovalsRequired  int             `json:"approvals_required"`
	ApprovalsLeft      int             `json:"approvals_left"`
	Approved           bool            `json:"approved"`
	ApprovedBy         []ApprovalVoter `json:"approved_by"`
	SuggestedApprovers []User          `json:"suggested_approvers"`
}

// AwardEmoji 是一条表情回应。
type AwardEmoji struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	User          *User  `json:"user"`
	AwardableID   int    `json:"awardable_id"`
	AwardableType string `json:"awardable_type"`
}

// mrBasePath 拼合并请求子资源路径。
func (c *Client) mrBasePath(projectID string, iid int) (string, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return "", err
	}
	if iid <= 0 {
		return "", invalidConfig("merge request iid is required")
	}
	return base + "/merge_requests/" + strconv.Itoa(iid), nil
}

// MRApprovals 读取审批状态。老实例或权限不足时由调用方降级为空。
func (c *Client) MRApprovals(ctx context.Context, projectID string, iid int) (*MRApprovals, error) {
	path, err := c.mrBasePath(projectID, iid)
	if err != nil {
		return nil, err
	}

	var approvals MRApprovals
	if _, err := c.getJSON(ctx, path+"/approvals", nil, &approvals); err != nil {
		return nil, err
	}
	return &approvals, nil
}

// MRCommits 列出合并请求范围内的提交（相对目标分支的差异提交）。
// 不要用 repository/commits?ref_name=源分支，那会带上整条分支历史。
func (c *Client) MRCommits(
	ctx context.Context,
	projectID string,
	iid, page, perPage int,
) (*CommitPage, error) {
	path, err := c.mrBasePath(projectID, iid)
	if err != nil {
		return nil, err
	}

	var items []Commit
	header, err := c.getJSON(ctx, path+"/commits", pagingQuery(page, perPage), &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, total := pageFromHeader(header)
	return &CommitPage{
		Items:   ensureSlice(items),
		Page:    current,
		HasMore: hasMore,
		Total:   total,
	}, nil
}

// MRAwardEmoji 列出合并请求上的表情回应。
func (c *Client) MRAwardEmoji(ctx context.Context, projectID string, iid int) ([]AwardEmoji, error) {
	path, err := c.mrBasePath(projectID, iid)
	if err != nil {
		return nil, err
	}

	var items []AwardEmoji
	if _, err := c.getJSON(ctx, path+"/award_emoji", pagingQuery(1, 100), &items); err != nil {
		return nil, err
	}
	return ensureSlice(items), nil
}

// AddMRAwardEmoji 给合并请求加一条表情回应。
func (c *Client) AddMRAwardEmoji(
	ctx context.Context,
	projectID string,
	iid int,
	name string,
) (*AwardEmoji, error) {
	path, err := c.mrBasePath(projectID, iid)
	if err != nil {
		return nil, err
	}
	emoji := strings.TrimSpace(name)
	if emoji == "" {
		return nil, invalidConfig("emoji name is required")
	}

	query := url.Values{}
	query.Set("name", emoji)

	var award AwardEmoji
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		path+"/award_emoji",
		query,
		nil,
		&award,
	); err != nil {
		return nil, err
	}
	return &award, nil
}

// RemoveMRAwardEmoji 取消一条表情回应。
func (c *Client) RemoveMRAwardEmoji(
	ctx context.Context,
	projectID string,
	iid, awardID int,
) error {
	path, err := c.mrBasePath(projectID, iid)
	if err != nil {
		return err
	}
	if awardID <= 0 {
		return invalidConfig("award emoji id is required")
	}

	_, err = c.writeJSON(
		ctx,
		http.MethodDelete,
		path+"/award_emoji/"+strconv.Itoa(awardID),
		nil,
		nil,
		nil,
	)
	return err
}
