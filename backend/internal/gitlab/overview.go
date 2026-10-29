package gitlab

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 单次目录浏览最多补多少条「最后提交」；GitLab 网页用 GraphQL 一次拿全，
// REST 只能按路径逐个查，这里给一个上限避免把实例打满。
const (
	maxTreeCommitPaths = 60
	treeCommitWorkers  = 6
)

// Branch 是仓库分支摘要。
type Branch struct {
	Name      string  `json:"name"`
	Default   bool    `json:"default"`
	Protected bool    `json:"protected"`
	Commit    *Commit `json:"commit,omitempty"`
}

// BranchPage 是分支分页结果。
type BranchPage struct {
	Items   []Branch `json:"items"`
	Page    int      `json:"page"`
	HasMore bool     `json:"has_more"`
}

// PathCommit 是单个文件/目录最近一次提交。
type PathCommit struct {
	Path   string  `json:"path"`
	Commit *Commit `json:"commit,omitempty"`
}

// RepoOverview 是项目页「项目信息」栏的聚合数据。
// 计数为 -1 表示实例没有返回 X-Total，UI 应显示「-」而不是 0。
type RepoOverview struct {
	BranchCount    int       `json:"branch_count"`
	TagCount       int       `json:"tag_count"`
	CommitCount    int       `json:"commit_count"`
	StorageSize    int64     `json:"storage_size"`
	RepositorySize int64     `json:"repository_size"`
	CreatedAt      time.Time `json:"created_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
	HeadCommit     *Commit   `json:"head_commit,omitempty"`
}

// Branches 列出仓库分支。
func (c *Client) Branches(
	ctx context.Context,
	projectID, search string,
	page, perPage int,
) (*BranchPage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	query := pagingQuery(page, perPage)
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		query.Set("search", trimmed)
	}

	var items []Branch
	header, err := c.getJSON(ctx, base+"/repository/branches", query, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &BranchPage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}

// RepoOverview 汇总项目信息栏所需的计数与最新提交。
// 统计/分支/标签对权限敏感，单项失败只降级为「未知」，不影响整页。
func (c *Client) RepoOverview(ctx context.Context, projectID, ref string) (*RepoOverview, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("statistics", "true")
	var project Project
	if _, err := c.getJSON(ctx, base, query, &project); err != nil {
		return nil, err
	}

	overview := &RepoOverview{
		BranchCount:    -1,
		TagCount:       -1,
		CommitCount:    -1,
		CreatedAt:      project.CreatedAt,
		LastActivityAt: project.LastActivityAt,
	}
	if stats := project.Statistics; stats != nil {
		overview.CommitCount = stats.CommitCount
		overview.StorageSize = stats.StorageSize
		overview.RepositorySize = stats.RepositorySize
	}

	branch := strings.TrimSpace(ref)
	if branch == "" {
		branch = project.DefaultBranch
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		overview.BranchCount = c.collectionCount(ctx, base+"/repository/branches")
	}()
	go func() {
		defer wg.Done()
		overview.TagCount = c.collectionCount(ctx, base+"/repository/tags")
	}()
	go func() {
		defer wg.Done()
		if commit, err := c.headCommit(ctx, base, branch); err == nil {
			overview.HeadCommit = commit
		}
	}()
	wg.Wait()

	return overview, nil
}

// TreeCommits 批量读取路径的最近一次提交，结果顺序与入参一致。
// 单个路径失败只留空，避免一个异常文件导致整页报错。
func (c *Client) TreeCommits(
	ctx context.Context,
	projectID, ref string,
	paths []string,
) ([]PathCommit, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	cleaned := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, raw := range paths {
		trimmed := strings.Trim(strings.TrimSpace(raw), "/")
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cleaned = append(cleaned, trimmed)
		if len(cleaned) >= maxTreeCommitPaths {
			break
		}
	}

	results := make([]PathCommit, len(cleaned))
	for i, path := range cleaned {
		results[i] = PathCommit{Path: path}
	}
	if len(cleaned) == 0 {
		return results, nil
	}

	workers := treeCommitWorkers
	if len(cleaned) < workers {
		workers = len(cleaned)
	}

	indexes := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range indexes {
				commit, err := c.pathCommit(ctx, base, ref, cleaned[index])
				if err != nil {
					continue
				}
				results[index].Commit = commit
			}
		}()
	}

	for index := range cleaned {
		select {
		case <-ctx.Done():
			close(indexes)
			wg.Wait()
			return results, ctx.Err()
		case indexes <- index:
		}
	}
	close(indexes)
	wg.Wait()

	return results, nil
}

// collectionCount 用 per_page=1 的响应头读总数；没有 X-Total 时返回 -1。
func (c *Client) collectionCount(ctx context.Context, path string) int {
	query := url.Values{}
	query.Set("per_page", "1")
	header, err := c.getJSON(ctx, path, query, nil)
	if err != nil {
		return -1
	}
	raw := strings.TrimSpace(header.Get("X-Total"))
	if raw == "" {
		return -1
	}
	total, err := strconv.Atoi(raw)
	if err != nil || total < 0 {
		return -1
	}
	return total
}

// headCommit 读取指定 ref 的最新提交。
func (c *Client) headCommit(ctx context.Context, base, ref string) (*Commit, error) {
	return c.pathCommit(ctx, base, ref, "")
}

// pathCommit 读取某个路径（空串表示整个仓库）的最新提交。
func (c *Client) pathCommit(ctx context.Context, base, ref, path string) (*Commit, error) {
	query := url.Values{}
	query.Set("per_page", "1")
	if trimmed := strings.TrimSpace(ref); trimmed != "" {
		query.Set("ref_name", trimmed)
	}
	if trimmed := strings.Trim(strings.TrimSpace(path), "/"); trimmed != "" {
		query.Set("path", trimmed)
	}

	var items []Commit
	if _, err := c.getJSON(ctx, base+"/repository/commits", query, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}
