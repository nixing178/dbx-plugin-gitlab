package gitlab

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// defaultMaxArtifactBytes 限制单次制品下载，避免把大 zip 全塞进内存。
const defaultMaxArtifactBytes = 8 << 20

// ProjectMembers 列出项目成员（包含从上层组继承的成员）。
// GitLab 不返回成员来源，这里用「直接成员清单」做差集标记 direct / inherited；
// 差集取不到时 Source 留空，列表照常返回。
func (c *Client) ProjectMembers(
	ctx context.Context,
	projectID, query string,
	page, perPage int,
) (*MemberPage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	params := pagingQuery(page, perPage)
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		params.Set("query", trimmed)
	}

	var items []Member
	header, err := c.getJSON(ctx, base+"/members/all", params, &items)
	if err != nil {
		return nil, err
	}
	items = ensureSlice(items)

	if len(items) > 0 {
		if direct, err := c.directMemberIDs(ctx, base); err == nil {
			for index := range items {
				if _, ok := direct[items[index].ID]; ok {
					items[index].Source = "direct"
				} else {
					items[index].Source = "inherited"
				}
			}
		}
	}

	current, hasMore, _ := pageFromHeader(header)
	return &MemberPage{Items: items, Page: current, HasMore: hasMore}, nil
}

// directMemberIDs 拉取项目直属成员（不含继承）的 id 集合。
// 成员量通常很小，最多翻 maxDirectMemberPages 页，避免异常数据导致长循环。
func (c *Client) directMemberIDs(ctx context.Context, base string) (map[int]struct{}, error) {
	const maxDirectMemberPages = 10

	ids := make(map[int]struct{})
	for page := 1; page <= maxDirectMemberPages; page += 1 {
		var items []Member
		header, err := c.getJSON(ctx, base+"/members", pagingQuery(page, 100), &items)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			ids[item.ID] = struct{}{}
		}
		if _, hasMore, _ := pageFromHeader(header); !hasMore {
			break
		}
	}
	return ids, nil
}

// ProjectMember 查询单个成员在项目里的有效角色（包含从上层组继承的权限）。
// 用于判断当前登录账号能管理哪些成员。
func (c *Client) ProjectMember(ctx context.Context, projectID string, userID int) (*Member, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, invalidConfig("user id is required")
	}

	var member Member
	if _, err := c.getJSON(
		ctx,
		base+"/members/all/"+strconv.Itoa(userID),
		nil,
		&member,
	); err != nil {
		return nil, err
	}
	return &member, nil
}

// UpdateMemberRole 修改项目直接成员的角色。
// 从上层组继承的成员 GitLab 会拒绝该请求，需到组内调整。
func (c *Client) UpdateMemberRole(
	ctx context.Context,
	projectID string,
	userID, accessLevel int,
) (*Member, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, invalidConfig("user id is required")
	}
	if accessLevel <= 0 {
		return nil, invalidConfig("access level is required")
	}

	var member Member
	if _, err := c.writeJSON(
		ctx,
		http.MethodPut,
		base+"/members/"+strconv.Itoa(userID),
		nil,
		map[string]any{"access_level": accessLevel},
		&member,
	); err != nil {
		return nil, err
	}
	member.Source = "direct"
	return &member, nil
}

// UpdateMemberExpiry 修改成员的过期时间；expiresAt 为空表示清除到期限制（发 null）。
// access_level 一并带上，避免部分实例把它当必填字段。
func (c *Client) UpdateMemberExpiry(
	ctx context.Context,
	projectID string,
	userID, accessLevel int,
	expiresAt string,
) (*Member, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, invalidConfig("user id is required")
	}
	if accessLevel <= 0 {
		return nil, invalidConfig("access level is required")
	}

	payload := map[string]any{"access_level": accessLevel, "expires_at": nil}
	if trimmed := strings.TrimSpace(expiresAt); trimmed != "" {
		payload["expires_at"] = trimmed
	}

	var member Member
	if _, err := c.writeJSON(
		ctx,
		http.MethodPut,
		base+"/members/"+strconv.Itoa(userID),
		nil,
		payload,
		&member,
	); err != nil {
		return nil, err
	}
	member.Source = "direct"
	return &member, nil
}

// RemoveMember 把成员移出项目（仅直接成员）。
func (c *Client) RemoveMember(ctx context.Context, projectID string, userID int) error {
	base, err := projectPath(projectID)
	if err != nil {
		return err
	}
	if userID <= 0 {
		return invalidConfig("user id is required")
	}

	_, err = c.writeJSON(
		ctx,
		http.MethodDelete,
		base+"/members/"+strconv.Itoa(userID),
		nil,
		nil,
		nil,
	)
	return err
}

// AddMemberOptions 是按用户名添加项目成员的参数。
type AddMemberOptions struct {
	Username    string
	AccessLevel int
	ExpiresAt   string
}

// InviteMemberOptions 是按邮箱邀请成员的参数。
type InviteMemberOptions struct {
	Email       string
	AccessLevel int
	ExpiresAt   string
}

// SearchUsers 按姓名或用户名搜索实例内用户，用于邀请成员时的联想。
func (c *Client) SearchUsers(ctx context.Context, search string, perPage int) ([]User, error) {
	trimmed := strings.TrimSpace(search)
	if trimmed == "" {
		return nil, invalidConfig("search query is required")
	}

	params := pagingQuery(1, perPage)
	params.Set("search", trimmed)

	var users []User
	if _, err := c.getJSON(ctx, "/users", params, &users); err != nil {
		return nil, err
	}
	return ensureSlice(users), nil
}

// AddProjectMember 把已有用户按用户名加入项目。
func (c *Client) AddProjectMember(
	ctx context.Context,
	projectID string,
	opts AddMemberOptions,
) (*Member, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	username := strings.TrimSpace(opts.Username)
	if username == "" {
		return nil, invalidConfig("username is required")
	}
	if opts.AccessLevel <= 0 {
		return nil, invalidConfig("access level is required")
	}

	payload := map[string]any{"username": username, "access_level": opts.AccessLevel}
	if expires := strings.TrimSpace(opts.ExpiresAt); expires != "" {
		payload["expires_at"] = expires
	}

	var member Member
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/members",
		nil,
		payload,
		&member,
	); err != nil {
		return nil, err
	}
	member.Source = "direct"
	return &member, nil
}

// InviteProjectMember 按邮箱邀请尚未加入项目的账号。
func (c *Client) InviteProjectMember(
	ctx context.Context,
	projectID string,
	opts InviteMemberOptions,
) (*Member, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	email := strings.TrimSpace(opts.Email)
	if email == "" {
		return nil, invalidConfig("email is required")
	}
	if opts.AccessLevel <= 0 {
		return nil, invalidConfig("access level is required")
	}

	payload := map[string]any{"email": email, "access_level": opts.AccessLevel}
	if expires := strings.TrimSpace(opts.ExpiresAt); expires != "" {
		payload["expires_at"] = expires
	}

	var member Member
	if _, err := c.writeJSON(
		ctx,
		http.MethodPost,
		base+"/invitations",
		nil,
		payload,
		&member,
	); err != nil {
		return nil, err
	}
	member.Source = "direct"
	return &member, nil
}

// SearchBlobs 在项目默认分支内搜索代码。
func (c *Client) SearchBlobs(
	ctx context.Context,
	projectID, search string,
	page, perPage int,
) (*SearchPage, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(search)
	if trimmed == "" {
		return nil, invalidConfig("search query is required")
	}

	params := pagingQuery(page, perPage)
	params.Set("scope", "blobs")
	params.Set("search", trimmed)

	var items []SearchResult
	header, err := c.getJSON(ctx, base+"/search", params, &items)
	if err != nil {
		return nil, err
	}
	current, hasMore, _ := pageFromHeader(header)
	return &SearchPage{Items: ensureSlice(items), Page: current, HasMore: hasMore}, nil
}

// PipelineTestReport 读取流水线的测试报告。
func (c *Client) PipelineTestReport(
	ctx context.Context,
	projectID string,
	pipelineID int,
) (*TestReport, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if pipelineID <= 0 {
		return nil, invalidConfig("pipeline id is required")
	}

	var report TestReport
	if _, err := c.getJSON(
		ctx,
		base+"/pipelines/"+strconv.Itoa(pipelineID)+"/test_report",
		nil,
		&report,
	); err != nil {
		return nil, err
	}
	return &report, nil
}

// JobArtifacts 下载作业制品；超过 maxBytes 时只返回大小。
func (c *Client) JobArtifacts(
	ctx context.Context,
	projectID string,
	jobID int,
	maxBytes int64,
) (*ArtifactDownload, error) {
	base, err := projectPath(projectID)
	if err != nil {
		return nil, err
	}
	if jobID <= 0 {
		return nil, invalidConfig("job id is required")
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxArtifactBytes
	}

	if err := c.ensureAuthorized(ctx); err != nil {
		return nil, err
	}

	endpoint := c.baseURL + "/api/v4" + base + "/jobs/" + strconv.Itoa(jobID) + "/artifacts"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, invalidConfig("cannot build request: %v", err)
	}
	c.applyAuth(request)
	request.Header.Set("User-Agent", userAgent)

	response, err := c.http.Do(request)
	if err != nil {
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
		return nil, mapHTTPError(response, payload)
	}

	filename := parseContentDispositionFilename(response.Header.Get("Content-Disposition"))

	size, _ := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if size > maxBytes {
		return &ArtifactDownload{Filename: filename, Size: size, TooLarge: true}, nil
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, &APIError{Kind: ErrNetwork, Message: err.Error()}
	}
	if int64(len(payload)) > maxBytes {
		return &ArtifactDownload{
			Filename: filename,
			Size:     int64(len(payload)),
			TooLarge: true,
		}, nil
	}

	if filename == "" {
		filename = "job-" + strconv.Itoa(jobID) + "-artifacts.zip"
	}

	return &ArtifactDownload{
		Filename: filename,
		Size:     int64(len(payload)),
		Encoding: "base64",
		Data:     base64.StdEncoding.EncodeToString(payload),
	}, nil
}

// parseContentDispositionFilename 从 Content-Disposition 里取文件名。
func parseContentDispositionFilename(value string) string {
	for _, segment := range strings.Split(value, ";") {
		trimmed := strings.TrimSpace(segment)
		if !strings.HasPrefix(trimmed, "filename=") {
			continue
		}
		name := strings.TrimPrefix(trimmed, "filename=")
		return strings.Trim(name, `"`)
	}
	return ""
}
