package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ProjectListOptions 项目列表的筛选与排序条件。
// Starred / Owned / 默认（membership）三者互斥，对应 GitLab 项目看板的三个页签。
type ProjectListOptions struct {
	Search             string
	Page               int
	PerPage            int
	Starred            bool
	Owned              bool
	OrderBy            string
	Sort               string
	LastActivityBefore string
}

// Projects 列出项目。
func (c *Client) Projects(ctx context.Context, opts ProjectListOptions) (*ProjectPage, error) {
	query := pagingQuery(opts.Page, opts.PerPage)
	switch {
	case opts.Starred:
		query.Set("starred", "true")
	case opts.Owned:
		query.Set("owned", "true")
	default:
		query.Set("membership", "true")
	}

	orderBy := strings.TrimSpace(opts.OrderBy)
	if orderBy == "" {
		orderBy = "last_activity_at"
	}
	sortOrder := strings.ToLower(strings.TrimSpace(opts.Sort))
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	query.Set("order_by", orderBy)
	query.Set("sort", sortOrder)

	if trimmed := strings.TrimSpace(opts.Search); trimmed != "" {
		query.Set("search", trimmed)
	}
	if before := strings.TrimSpace(opts.LastActivityBefore); before != "" {
		query.Set("last_activity_before", before)
	}

	var items []Project
	header, err := c.getJSON(ctx, "/projects", query, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, total := pageFromHeader(header)
	return &ProjectPage{
		Items:   ensureSlice(items),
		Page:    current,
		HasMore: hasMore,
		Total:   total,
	}, nil
}

// Project 读取单个项目详情。
// 星标状态不在这里查：GitLab 的详情不返回 starred，查询要走 GraphQL 或列表，
// 放进详情会拖慢甚至拖挂整个项目页；前端用独立的 gitlab/projectStarred 异步补。
func (c *Client) Project(ctx context.Context, projectID string) (*Project, error) {
	path, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	var project Project
	if _, err := c.getJSON(ctx, path, nil, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// ProjectStarred 查询当前用户是否已给项目加星。
// 优先 GraphQL viewerHasStarred，失败退回 starred 列表；总耗时限制 8 秒，
// 避免实例不支持 GraphQL 时把项目页拖住。
func (c *Client) ProjectStarred(ctx context.Context, projectID string) (bool, error) {
	fullPath := strings.TrimSpace(projectID)
	if fullPath == "" {
		return false, invalidConfig("project id is required")
	}
	if !strings.Contains(fullPath, "/") {
		project, err := c.Project(ctx, fullPath)
		if err != nil {
			return false, err
		}
		fullPath = project.PathWithNamespace
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	if starred, ok := c.viewerHasStarred(lookupCtx, fullPath); ok {
		return starred, nil
	}
	return c.starredFromList(lookupCtx, fullPath), nil
}

// starredFromList 在「当前用户已加星」的项目里精确匹配完整路径。
// 不能带 search：GitLab 的 search 只匹配项目名/路径片段，传 "group/project" 反而查不到。
func (c *Client) starredFromList(ctx context.Context, fullPath string) bool {
	target := strings.TrimSpace(fullPath)
	if target == "" {
		return false
	}

	for page := 1; page <= 5; page++ {
		query := url.Values{}
		query.Set("starred", "true")
		query.Set("per_page", "100")
		query.Set("page", strconv.Itoa(page))

		var items []Project
		header, err := c.getJSON(ctx, "/projects", query, &items)
		if err != nil {
			return false
		}
		for _, item := range items {
			if strings.EqualFold(item.PathWithNamespace, target) {
				return true
			}
		}
		if strings.TrimSpace(header.Get("X-Next-Page")) == "" {
			return false
		}
	}
	return false
}

// StarResult 是加星/取消星标的结果。
// StarCount = -1 表示 GitLab 返回 304（状态本来就一致），前端保持本地计数不变。
type StarResult struct {
	Starred   bool `json:"starred"`
	StarCount int  `json:"star_count"`
}

// StarProject 给项目加星（starred=true）或取消星标。
// GitLab 的取消星标是 POST /projects/:id/unstar（DELETE /star 会 404）；
// 重复操作返回 304 Not Modified，这里按幂等成功处理。
func (c *Client) StarProject(ctx context.Context, projectID string, starred bool) (*StarResult, error) {
	path, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	target := path + "/unstar"
	if starred {
		target = path + "/star"
	}

	var project Project
	if _, err := c.writeJSON(ctx, http.MethodPost, target, nil, nil, &project); err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.Status == http.StatusNotModified {
			return &StarResult{Starred: starred, StarCount: -1}, nil
		}
		return nil, err
	}
	return &StarResult{Starred: starred, StarCount: project.StarCount}, nil
}
