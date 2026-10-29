package gitlab

import (
	"context"
	"net/url"
	"strings"
)

// CompareFile 是分支比较里的单文件统计。
//
// GitLab 的 compare 会一次性返回全部 diff 正文，大仓库可能几十 MB，
// 而新建合并请求页只需要文件清单与增删行数，所以正文在返回前丢弃。
type CompareFile struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
	TooLarge    bool   `json:"too_large"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
	// Diff 只在解码阶段接收正文，统计完成后清空，序列化时省略。
	Diff string `json:"diff,omitempty"`
}

// CompareResult 是两个 ref 之间的比较结果。
type CompareResult struct {
	Commits        []Commit      `json:"commits"`
	Diffs          []CompareFile `json:"diffs"`
	CompareTimeout bool          `json:"compare_timeout"`
	CompareSameRef bool          `json:"compare_same_ref"`
}

// Compare 比较两个 ref，返回 from..to 之间的提交与文件统计。
//
// from 传目标分支、to 传源分支，与 GitLab 网页新建 MR 页一致；
// straight=true 表示只取两点之间的差异，不跟随 merge base 展开。
func (c *Client) Compare(
	ctx context.Context,
	projectID, from, to string,
) (*CompareResult, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || to == "" {
		return nil, invalidConfig("from and to refs are required")
	}

	query := url.Values{}
	query.Set("from", from)
	query.Set("to", to)
	query.Set("straight", "true")

	var result CompareResult
	if _, err := c.getJSON(ctx, base+"/repository/compare", query, &result); err != nil {
		return nil, err
	}

	files := ensureSlice(result.Diffs)
	for i := range files {
		files[i].Additions, files[i].Deletions = countDiffLines(files[i].Diff)
		files[i].Diff = ""
	}

	return &CompareResult{
		Commits:        ensureSlice(result.Commits),
		Diffs:          files,
		CompareTimeout: result.CompareTimeout,
		CompareSameRef: result.CompareSameRef,
	}, nil
}

// countDiffLines 统计 unified diff 的增删行数，跳过 +++ / --- 文件头。
func countDiffLines(diff string) (additions, deletions int) {
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"):
			additions++
		case strings.HasPrefix(line, "-"):
			deletions++
		}
	}
	return additions, deletions
}
