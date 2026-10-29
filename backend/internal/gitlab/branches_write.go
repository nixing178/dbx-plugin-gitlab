package gitlab

import (
	"context"
	"net/http"
	"strings"
)

// CreateBranch 从现有分支、标签或提交 SHA 创建分支。
// GitLab 对非法分支名/已存在分支返回 400，错误信息原样交给 UI 展示。
func (c *Client) CreateBranch(
	ctx context.Context,
	projectID, branch, ref string,
) (*Branch, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(branch)
	start := strings.TrimSpace(ref)
	if name == "" {
		return nil, invalidConfig("branch name is required")
	}
	if start == "" {
		return nil, invalidConfig("ref is required")
	}

	var created Branch
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/repository/branches",
		nil,
		map[string]any{"branch": name, "ref": start},
		&created,
	); err != nil {
		return nil, err
	}
	return &created, nil
}
