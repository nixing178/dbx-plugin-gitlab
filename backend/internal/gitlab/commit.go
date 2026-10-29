package gitlab

import (
	"context"
	"net/url"
	"strings"
)

// CommitRef 是包含该提交的分支或标签。
type CommitRef struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// CommitDetail 是提交详情页的聚合数据。
type CommitDetail struct {
	Commit        *Commit     `json:"commit"`
	Refs          []CommitRef `json:"refs"`
	MergeRequests []MR        `json:"merge_requests"`
}

// CommitDetail 读取提交详情、包含它的分支/标签以及关联合并请求。
// refs / merge_requests 对实例版本和权限敏感，取不到时降级为空，不影响提交本身。
func (c *Client) CommitDetail(ctx context.Context, projectID, sha string) (*CommitDetail, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	trimmedSHA := strings.TrimSpace(sha)
	if trimmedSHA == "" {
		return nil, invalidConfig("commit sha is required")
	}

	commitPath := base + "/repository/commits/" + url.PathEscape(trimmedSHA)

	var commit Commit
	if _, err := c.getJSON(ctx, commitPath, nil, &commit); err != nil {
		return nil, err
	}

	detail := &CommitDetail{Commit: &commit, Refs: []CommitRef{}, MergeRequests: []MR{}}

	refsQuery := url.Values{}
	refsQuery.Set("type", "all")
	var refs []CommitRef
	if _, err := c.getJSON(ctx, commitPath+"/refs", refsQuery, &refs); err == nil {
		detail.Refs = ensureSlice(refs)
	}

	var mergeRequests []MR
	if _, err := c.getJSON(ctx, commitPath+"/merge_requests", pagingQuery(1, 20), &mergeRequests); err == nil {
		detail.MergeRequests = ensureSlice(mergeRequests)
	}

	return detail, nil
}
