package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// MROptions 是合并请求列表的筛选条件。
type MROptions struct {
	Scope   string
	State   string
	Search  string
	OrderBy string
	Sort    string
	Page    int
	PerPage int
}

// MergeRequests 列出合并请求。
// projectID 为空时走全局视角（需要 Scope），否则走项目视角。
func (c *Client) MergeRequests(
	ctx context.Context,
	projectID string,
	opts MROptions,
) (*MRPage, error) {
	query := pagingQuery(opts.Page, opts.PerPage)
	if trimmed := strings.TrimSpace(opts.State); trimmed != "" {
		query.Set("state", trimmed)
	}
	if trimmed := strings.TrimSpace(opts.Search); trimmed != "" {
		query.Set("search", trimmed)
	}
	if trimmed := strings.TrimSpace(opts.OrderBy); trimmed != "" {
		query.Set("order_by", trimmed)
	}
	if trimmed := strings.ToLower(strings.TrimSpace(opts.Sort)); trimmed != "" {
		query.Set("sort", trimmed)
	}

	path, err := c.mrListPath(ctx, projectID, opts.Scope, query)
	if err != nil {
		return nil, err
	}

	var items []MR
	header, err := c.getJSON(ctx, path, query, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, total := pageFromHeader(header)
	return &MRPage{Items: ensureSlice(items), Page: current, HasMore: hasMore, Total: total}, nil
}

// mrListPath 决定列表路径，并把全局视角的 scope 写入 query。
// GitLab 17.x 的全局 MR 接口没有 reviewer scope（reviews_for_me 是 18.x 才引入的），
// 「待我评审」用 scope=all + reviewer_id 表达。
func (c *Client) mrListPath(
	ctx context.Context,
	projectID, scope string,
	query url.Values,
) (string, error) {
	if strings.TrimSpace(projectID) != "" {
		base, err := projectPath(projectID)
		if err != nil {
			return "", err
		}
		return base + "/merge_requests", nil
	}

	trimmed := strings.TrimSpace(scope)
	if trimmed == "" {
		return "/merge_requests", nil
	}
	if trimmed == "reviewer" {
		user, err := c.CurrentUser(ctx)
		if err != nil {
			return "", err
		}
		query.Set("scope", "all")
		query.Set("reviewer_id", strconv.Itoa(user.ID))
		return "/merge_requests", nil
	}
	query.Set("scope", trimmed)
	return "/merge_requests", nil
}

// mrCountStates 是页签要读的状态；空串代表「全部」。
var mrCountStates = []string{"opened", "merged", "closed", ""}

// MRCounts 聚合页签上的四个计数。
// 每次只取 per_page=1 的响应头 X-Total；单项失败只把该项标成 -1，不阻塞列表。
func (c *Client) MRCounts(
	ctx context.Context,
	projectID string,
	opts MROptions,
) (*MRStateCounts, error) {
	baseQuery := url.Values{}
	path, err := c.mrListPath(ctx, projectID, opts.Scope, baseQuery)
	if err != nil {
		return nil, err
	}
	if trimmed := strings.TrimSpace(opts.Search); trimmed != "" {
		baseQuery.Set("search", trimmed)
	}

	results := make([]int, len(mrCountStates))
	var wait sync.WaitGroup
	for index, state := range mrCountStates {
		wait.Add(1)
		go func(index int, state string) {
			defer wait.Done()
			results[index] = c.mergeRequestCount(ctx, path, baseQuery, state)
		}(index, state)
	}
	wait.Wait()

	return &MRStateCounts{
		Opened: results[0],
		Merged: results[1],
		Closed: results[2],
		All:    results[3],
	}, nil
}

// mergeRequestCount 用 per_page=1 的响应头读总数；请求失败或没有 X-Total 时返回 -1。
func (c *Client) mergeRequestCount(
	ctx context.Context,
	path string,
	baseQuery url.Values,
	state string,
) int {
	query := cloneValues(baseQuery)
	query.Set("page", "1")
	query.Set("per_page", "1")
	if state != "" {
		query.Set("state", state)
	}

	header, err := c.getJSON(ctx, path, query, nil)
	if err != nil {
		return -1
	}
	raw := strings.TrimSpace(header.Get("X-Total"))
	if raw == "" {
		return -1
	}
	total, convErr := strconv.Atoi(raw)
	if convErr != nil || total < 0 {
		return -1
	}
	return total
}

// cloneValues 复制查询参数，让并发请求各持一份。
func cloneValues(source url.Values) url.Values {
	target := make(url.Values, len(source))
	for key, values := range source {
		target[key] = append([]string(nil), values...)
	}
	return target
}

// MergeRequest 读取单个合并请求详情。
func (c *Client) MergeRequest(ctx context.Context, projectID string, iid int) (*MR, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}

	var mr MR
	if _, err := c.getJSON(ctx, base+"/merge_requests/"+strconv.Itoa(iid), nil, &mr); err != nil {
		return nil, err
	}
	return &mr, nil
}

// mrDiffPayload 兼容 /diffs 的两种响应形状：
//   - 数组：[{old_path, new_path, diff, ...}]（GitLab 标准返回，不带 diff_refs）
//   - 对象：{changes: [...], diff_refs: {...}, overflow: false}（部分版本/端点）
//
// 早期实现只按对象解，遇到数组会整包报「格式错误」，变更页因此一直是空的。
type mrDiffPayload struct {
	DiffRefs DiffRefs
	Changes  []DiffFile
	Overflow bool
}

func (p *mrDiffPayload) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return json.Unmarshal(data, &p.Changes)
	}

	var envelope struct {
		DiffRefs DiffRefs   `json:"diff_refs"`
		Changes  []DiffFile `json:"changes"`
		Overflow bool       `json:"overflow"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	p.DiffRefs = envelope.DiffRefs
	p.Changes = envelope.Changes
	p.Overflow = envelope.Overflow
	return nil
}

// MRDiffs 读取合并请求的差异分页。
// 数组响应里没有 diff_refs，调用方要用 MR 详情上的 diff_refs 做行内评论定位。
func (c *Client) MRDiffs(
	ctx context.Context,
	projectID string,
	iid, page, perPage int,
) (*MRDiffPage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}

	var payload mrDiffPayload
	header, err := c.getJSONLimited(
		ctx,
		base+"/merge_requests/"+strconv.Itoa(iid)+"/diffs",
		pagingQuery(page, perPage),
		&payload,
		maxDiffBody,
	)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &MRDiffPage{
		Items:    ensureSlice(payload.Changes),
		Page:     current,
		HasMore:  hasMore,
		DiffRefs: payload.DiffRefs,
	}, nil
}

// MRDiscussions 读取合并请求的讨论线程。
func (c *Client) MRDiscussions(ctx context.Context, projectID string, iid int) ([]Discussion, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if iid <= 0 {
		return nil, invalidConfig("merge request iid is required")
	}

	var items []Discussion
	if _, err := c.getJSON(
		ctx,
		base+"/merge_requests/"+strconv.Itoa(iid)+"/discussions",
		nil,
		&items,
	); err != nil {
		return nil, err
	}
	return ensureSlice(items), nil
}
