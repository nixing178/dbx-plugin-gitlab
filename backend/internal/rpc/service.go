package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nixing178/dbx-plugin-gitlab/internal/gitlab"
)

const pluginID = "io.github.nixing178.gitlab"

// Error 是 RPC 层的统一错误，main 会把它转换成 SDK 的 PluginError。
// Message 以 "<kind>: <detail>" 形式返回，UI 可据此做本地化。
type Error struct {
	Code    int
	Kind    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// LifecycleRequest 是 DBX 生命周期请求与业务参数的合集。
type LifecycleRequest struct {
	Connection struct {
		ID                   string         `json:"id"`
		Name                 string         `json:"name"`
		Host                 string         `json:"host"`
		Port                 int            `json:"port"`
		Username             string         `json:"username"`
		Password             string         `json:"password"`
		APIToken             string         `json:"api_token"`
		ReadOnly             bool           `json:"read_only"`
		ExternalConfig       map[string]any `json:"external_config"`
		ExternalConfigNew    map[string]any `json:"externalConfig"`
		Secrets              map[string]any `json:"secrets"`
		ConnectionSecrets    map[string]any `json:"connection_secrets"`
		ConnectionSecretsNew map[string]any `json:"connectionSecrets"`
	} `json:"connection"`
	Runtime struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"runtime"`
	Secrets      map[string]any `json:"secrets"`
	ConnectionID string         `json:"connectionId"`

	ProjectID string   `json:"projectId"`
	Path      string   `json:"path"`
	Paths     []string `json:"paths"`
	Ref       string   `json:"ref"`
	Branch    string   `json:"branch"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Search    string   `json:"search"`
	Page      int    `json:"page"`
	PerPage   int    `json:"perPage"`
	IID       int    `json:"iid"`
	SHA       string `json:"sha"`
	Scope     string `json:"scope"`
	State     string `json:"state"`

	// 项目列表筛选
	Starred            bool   `json:"starred"`
	Owned              bool   `json:"owned"`
	OrderBy            string `json:"orderBy"`
	Sort               string `json:"sort"`
	LastActivityBefore string `json:"lastActivityBefore"`

	// 写操作参数
	DiscussionID        string                `json:"discussionId"`
	Body                string                `json:"body"`
	Resolved            bool                  `json:"resolved"`
	Position            *gitlab.PositionInput `json:"position"`
	Squash              bool                  `json:"squash"`
	RemoveSourceBranch  bool                  `json:"removeSourceBranch"`
	MergeCommitMessage  string                `json:"mergeCommitMessage"`
	SquashCommitMessage string                `json:"squashCommitMessage"`
	StateEvent          string                `json:"stateEvent"`
	Title               string                `json:"title"`
	Description         string                `json:"description"`
	SourceBranch        string                `json:"sourceBranch"`
	TargetBranch        string                `json:"targetBranch"`
	AssigneeIDs         []int                 `json:"assigneeIds"`
	ReviewerIDs         []int                 `json:"reviewerIds"`
	Labels              []string              `json:"labels"`
	// MilestoneID 用指针区分「没传」与「传 0 清空里程碑」。
	MilestoneID *int `json:"milestoneId"`
	EmojiName   string `json:"emojiName"`
	AwardID     int    `json:"awardId"`
	// 成员写操作
	UserID      int    `json:"userId"`
	AccessLevel int    `json:"accessLevel"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	ExpiresAt   string `json:"expiresAt"`

	// CI 参数
	PipelineID int                       `json:"pipelineId"`
	JobID      int                       `json:"jobId"`
	Offset     int64                     `json:"offset"`
	Variables  []gitlab.PipelineVariable `json:"variables"`

	// Issue / Todo 参数
	TodoID  int    `json:"todoId"`
	DueDate string `json:"dueDate"`
}

// Service 管理每个连接对应的 GitLab 客户端。
type Service struct {
	mu       sync.RWMutex
	sessions map[string]*gitlab.Client
}

// NewService 创建 RPC 服务。
func NewService() *Service {
	return &Service{sessions: map[string]*gitlab.Client{}}
}

// Handle 分发一个 RPC 方法。
func (s *Service) Handle(ctx context.Context, method string, raw json.RawMessage) (any, *Error) {
	if method == "mcp/tools" {
		return s.handleMCPTools(raw)
	}
	if method == "mcp/call" {
		return s.handleMCPCall(ctx, raw)
	}

	var request LifecycleRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, invalidParams("invalid request parameters")
		}
	}

	switch method {
	case "connection/test":
		client, rpcErr := buildClient(request)
		if rpcErr != nil {
			return nil, rpcErr
		}
		defer client.CloseIdleConnections()

		info, err := client.TestConnection(ctx)
		if err != nil {
			return nil, mapError(err)
		}
		return connectionResult(client, info), nil

	case "connection/connect":
		connectionID := firstNonEmpty(request.Connection.ID, request.ConnectionID)
		if connectionID == "" {
			return nil, invalidParams("missing connection id")
		}
		client, rpcErr := buildClient(request)
		if rpcErr != nil {
			return nil, rpcErr
		}
		info, err := client.TestConnection(ctx)
		if err != nil {
			client.CloseIdleConnections()
			return nil, mapError(err)
		}

		s.mu.Lock()
		if existing := s.sessions[connectionID]; existing != nil {
			existing.CloseIdleConnections()
		}
		s.sessions[connectionID] = client
		s.mu.Unlock()

		return connectionResult(client, info), nil

	case "connection/disconnect":
		connectionID := firstNonEmpty(request.Connection.ID, request.ConnectionID)
		if connectionID == "" {
			return nil, invalidParams("missing connection id")
		}
		s.mu.Lock()
		if existing := s.sessions[connectionID]; existing != nil {
			existing.CloseIdleConnections()
		}
		delete(s.sessions, connectionID)
		s.mu.Unlock()
		return map[string]any{"success": true}, nil

	case "gitlab/whoami":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		user, err := client.WhoAmI(ctx)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{
			"user":      user,
			"read_only": client.ReadOnly(),
			"version":   client.Version(),
		}, nil

	case "gitlab/projects":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.Projects(ctx, gitlab.ProjectListOptions{
			Search:             request.Search,
			Page:               request.Page,
			PerPage:            request.PerPage,
			Starred:            request.Starred,
			Owned:              request.Owned,
			OrderBy:            request.OrderBy,
			Sort:               request.Sort,
			LastActivityBefore: request.LastActivityBefore,
		})
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/project":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		project, err := client.Project(ctx, request.ProjectID)
		if err != nil {
			return nil, mapError(err)
		}
		return project, nil

	case "gitlab/projectStarred":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		starred, err := client.ProjectStarred(ctx, request.ProjectID)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"starred": starred}, nil

	case "gitlab/starProject":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		result, err := client.StarProject(ctx, request.ProjectID, request.Starred)
		if err != nil {
			return nil, mapError(err)
		}
		return result, nil

	case "gitlab/tree":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.Tree(ctx, request.ProjectID, request.Path, request.Ref, request.Page, request.PerPage)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/file":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		file, err := client.File(ctx, request.ProjectID, request.Path, request.Ref)
		if err != nil {
			return nil, mapError(err)
		}
		return file, nil

	case "gitlab/commits":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.Commits(ctx, request.ProjectID, request.Ref, request.Page, request.PerPage)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/commitDiff":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		files, err := client.CommitDiff(ctx, request.ProjectID, request.SHA)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"items": files}, nil

	case "gitlab/commitDetail":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		detail, err := client.CommitDetail(ctx, request.ProjectID, request.SHA)
		if err != nil {
			return nil, mapError(err)
		}
		return detail, nil

	case "gitlab/branches":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.Branches(ctx, request.ProjectID, request.Search, request.Page, request.PerPage)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/createBranch":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		branch, err := client.CreateBranch(ctx, request.ProjectID, request.Branch, request.Ref)
		if err != nil {
			return nil, mapError(err)
		}
		return branch, nil

	case "gitlab/compare":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		compare, err := client.Compare(ctx, request.ProjectID, request.From, request.To)
		if err != nil {
			return nil, mapError(err)
		}
		return compare, nil

	case "gitlab/labels":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.Labels(
			ctx,
			request.ProjectID,
			request.Search,
			request.Page,
			request.PerPage,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/milestones":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.Milestones(
			ctx,
			request.ProjectID,
			request.State,
			request.Page,
			request.PerPage,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/repoOverview":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		overview, err := client.RepoOverview(ctx, request.ProjectID, request.Ref)
		if err != nil {
			return nil, mapError(err)
		}
		return overview, nil

	case "gitlab/treeCommits":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		items, err := client.TreeCommits(ctx, request.ProjectID, request.Ref, request.Paths)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"items": items}, nil

	case "gitlab/mrs":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.MergeRequests(ctx, request.ProjectID, gitlab.MROptions{
			Scope:   request.Scope,
			State:   request.State,
			Search:  request.Search,
			OrderBy: request.OrderBy,
			Sort:    request.Sort,
			Page:    request.Page,
			PerPage: request.PerPage,
		})
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/mrCounts":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		counts, err := client.MRCounts(ctx, request.ProjectID, gitlab.MROptions{
			Scope:  request.Scope,
			Search: request.Search,
		})
		if err != nil {
			return nil, mapError(err)
		}
		return counts, nil

	case "gitlab/mr":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		mr, err := client.MergeRequest(ctx, request.ProjectID, request.IID)
		if err != nil {
			return nil, mapError(err)
		}
		return mr, nil

	case "gitlab/mrDiffs":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.MRDiffs(ctx, request.ProjectID, request.IID, request.Page, request.PerPage)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/mrDiscussions":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		items, err := client.MRDiscussions(ctx, request.ProjectID, request.IID)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"items": items}, nil

	case "gitlab/mrVersions":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		items, err := client.MRVersions(ctx, request.ProjectID, request.IID)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"items": items}, nil

	case "gitlab/pipelines":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.Pipelines(ctx, request.ProjectID, gitlab.PipelineOptions{
			Ref:     request.Ref,
			Status:  request.State,
			Page:    request.Page,
			PerPage: request.PerPage,
		})
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/pipeline":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		detail, err := client.Pipeline(ctx, request.ProjectID, request.PipelineID)
		if err != nil {
			return nil, mapError(err)
		}
		return detail, nil

	case "gitlab/pipelineJobs":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.PipelineJobs(
			ctx,
			request.ProjectID,
			request.PipelineID,
			request.Page,
			request.PerPage,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/jobTrace":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		trace, err := client.JobTrace(ctx, request.ProjectID, request.JobID, request.Offset)
		if err != nil {
			return nil, mapError(err)
		}
		return trace, nil

	case "gitlab/retryJob":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		job, err := client.RetryJob(ctx, request.ProjectID, request.JobID)
		if err != nil {
			return nil, mapError(err)
		}
		return job, nil

	case "gitlab/cancelJob":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		job, err := client.CancelJob(ctx, request.ProjectID, request.JobID)
		if err != nil {
			return nil, mapError(err)
		}
		return job, nil

	case "gitlab/playJob":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		job, err := client.PlayJob(ctx, request.ProjectID, request.JobID)
		if err != nil {
			return nil, mapError(err)
		}
		return job, nil

	case "gitlab/retryPipeline":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		pipeline, err := client.RetryPipeline(ctx, request.ProjectID, request.PipelineID)
		if err != nil {
			return nil, mapError(err)
		}
		return pipeline, nil

	case "gitlab/cancelPipeline":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		pipeline, err := client.CancelPipeline(ctx, request.ProjectID, request.PipelineID)
		if err != nil {
			return nil, mapError(err)
		}
		return pipeline, nil

	case "gitlab/createPipeline":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		pipeline, err := client.CreatePipeline(
			ctx,
			request.ProjectID,
			request.Ref,
			request.Variables,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return pipeline, nil

	case "gitlab/commentMr":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		note, err := client.CommentMR(ctx, request.ProjectID, request.IID, request.Body, request.Position)
		if err != nil {
			return nil, mapError(err)
		}
		return note, nil

	case "gitlab/replyDiscussion":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		note, err := client.ReplyDiscussion(
			ctx,
			request.ProjectID,
			request.IID,
			request.DiscussionID,
			request.Body,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return note, nil

	case "gitlab/resolveDiscussion":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		discussion, err := client.ResolveDiscussion(
			ctx,
			request.ProjectID,
			request.IID,
			request.DiscussionID,
			request.Resolved,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return discussion, nil

	case "gitlab/mrCommits":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.MRCommits(
			ctx,
			request.ProjectID,
			request.IID,
			request.Page,
			request.PerPage,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/mrApprovals":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		approvals, err := client.MRApprovals(ctx, request.ProjectID, request.IID)
		if err != nil {
			return nil, mapError(err)
		}
		return approvals, nil

	case "gitlab/mrAwardEmoji":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		items, err := client.MRAwardEmoji(ctx, request.ProjectID, request.IID)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"items": items}, nil

	case "gitlab/addMrAwardEmoji":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		award, err := client.AddMRAwardEmoji(ctx, request.ProjectID, request.IID, request.EmojiName)
		if err != nil {
			return nil, mapError(err)
		}
		return award, nil

	case "gitlab/removeMrAwardEmoji":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		if err := client.RemoveMRAwardEmoji(ctx, request.ProjectID, request.IID, request.AwardID); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "gitlab/approveMr":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		approval, err := client.ApproveMR(ctx, request.ProjectID, request.IID, request.SHA)
		if err != nil {
			return nil, mapError(err)
		}
		return approval, nil

	case "gitlab/unapproveMr":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		if err := client.UnapproveMR(ctx, request.ProjectID, request.IID); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "gitlab/mergeMr":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		mr, err := client.MergeMR(ctx, request.ProjectID, request.IID, gitlab.MergeOptions{
			SHA:                 request.SHA,
			Squash:              request.Squash,
			RemoveSourceBranch:  request.RemoveSourceBranch,
			MergeCommitMessage:  request.MergeCommitMessage,
			SquashCommitMessage: request.SquashCommitMessage,
		})
		if err != nil {
			return nil, mapError(err)
		}
		return mr, nil

	case "gitlab/setMrState":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		mr, err := client.SetMRState(ctx, request.ProjectID, request.IID, request.StateEvent)
		if err != nil {
			return nil, mapError(err)
		}
		return mr, nil

	case "gitlab/createMr":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		milestoneID := 0
		if request.MilestoneID != nil {
			milestoneID = *request.MilestoneID
		}
		mr, err := client.CreateMR(ctx, request.ProjectID, gitlab.CreateMRInput{
			SourceBranch:       request.SourceBranch,
			TargetBranch:       request.TargetBranch,
			Title:              request.Title,
			Description:        request.Description,
			AssigneeIDs:        request.AssigneeIDs,
			ReviewerIDs:        request.ReviewerIDs,
			Labels:             request.Labels,
			MilestoneID:        milestoneID,
			RemoveSourceBranch: request.RemoveSourceBranch,
			Squash:             request.Squash,
		})
		if err != nil {
			return nil, mapError(err)
		}
		return mr, nil

	case "gitlab/updateMr":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		input := gitlab.UpdateMRInput{}
		if request.Title != "" {
			title := request.Title
			input.Title = &title
		}
		if request.Description != "" {
			description := request.Description
			input.Description = &description
		}
		if request.TargetBranch != "" {
			target := request.TargetBranch
			input.TargetBranch = &target
		}
		if request.AssigneeIDs != nil {
			input.AssigneeIDs = request.AssigneeIDs
		}
		if request.ReviewerIDs != nil {
			input.ReviewerIDs = request.ReviewerIDs
		}
		if request.Labels != nil {
			input.Labels = request.Labels
		}
		if request.MilestoneID != nil {
			input.MilestoneID = request.MilestoneID
		}
		mr, err := client.UpdateMR(ctx, request.ProjectID, request.IID, input)
		if err != nil {
			return nil, mapError(err)
		}
		return mr, nil

	case "gitlab/issues":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.Issues(ctx, gitlab.IssueOptions{
			ProjectID: request.ProjectID,
			Scope:     request.Scope,
			State:     request.State,
			Search:    request.Search,
			Labels:    strings.Join(request.Labels, ","),
			Page:      request.Page,
			PerPage:   request.PerPage,
		})
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/issue":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		issue, err := client.Issue(ctx, request.ProjectID, request.IID)
		if err != nil {
			return nil, mapError(err)
		}
		return issue, nil

	case "gitlab/issueNotes":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		items, err := client.IssueNotes(ctx, request.ProjectID, request.IID)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"items": items}, nil

	case "gitlab/todos":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.Todos(ctx, request.State, request.Page, request.PerPage)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/createIssue":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		issue, err := client.CreateIssue(ctx, request.ProjectID, gitlab.CreateIssueInput{
			Title:       request.Title,
			Description: request.Description,
			Labels:      request.Labels,
			AssigneeIDs: request.AssigneeIDs,
			DueDate:     request.DueDate,
		})
		if err != nil {
			return nil, mapError(err)
		}
		return issue, nil

	case "gitlab/updateIssue":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		input := gitlab.UpdateIssueInput{StateEvent: request.StateEvent}
		if request.Title != "" {
			title := request.Title
			input.Title = &title
		}
		if request.Description != "" {
			description := request.Description
			input.Description = &description
		}
		if request.Labels != nil {
			input.Labels = request.Labels
		}
		if request.AssigneeIDs != nil {
			input.AssigneeIDs = request.AssigneeIDs
		}
		issue, err := client.UpdateIssue(ctx, request.ProjectID, request.IID, input)
		if err != nil {
			return nil, mapError(err)
		}
		return issue, nil

	case "gitlab/commentIssue":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		note, err := client.CommentIssue(ctx, request.ProjectID, request.IID, request.Body)
		if err != nil {
			return nil, mapError(err)
		}
		return note, nil

	case "gitlab/markTodoDone":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		todo, err := client.MarkTodoDone(ctx, request.TodoID)
		if err != nil {
			return nil, mapError(err)
		}
		return todo, nil

	case "gitlab/projectMembers":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.ProjectMembers(
			ctx,
			request.ProjectID,
			request.Search,
			request.Page,
			request.PerPage,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/projectMember":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		member, err := client.ProjectMember(ctx, request.ProjectID, request.UserID)
		if err != nil {
			return nil, mapError(err)
		}
		return member, nil

	case "gitlab/updateMemberRole":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		member, err := client.UpdateMemberRole(
			ctx,
			request.ProjectID,
			request.UserID,
			request.AccessLevel,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return member, nil

	case "gitlab/updateMemberExpiry":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		member, err := client.UpdateMemberExpiry(
			ctx,
			request.ProjectID,
			request.UserID,
			request.AccessLevel,
			request.ExpiresAt,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return member, nil

	case "gitlab/removeMember":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		if err := client.RemoveMember(ctx, request.ProjectID, request.UserID); err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"success": true}, nil

	case "gitlab/searchUsers":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		users, err := client.SearchUsers(ctx, request.Search, request.PerPage)
		if err != nil {
			return nil, mapError(err)
		}
		return map[string]any{"items": users}, nil

	case "gitlab/addMember":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		member, err := client.AddProjectMember(ctx, request.ProjectID, gitlab.AddMemberOptions{
			Username:    request.Username,
			AccessLevel: request.AccessLevel,
			ExpiresAt:   request.ExpiresAt,
		})
		if err != nil {
			return nil, mapError(err)
		}
		return member, nil

	case "gitlab/inviteMember":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if guard := s.requireWritable(client); guard != nil {
			return nil, guard
		}
		member, err := client.InviteProjectMember(ctx, request.ProjectID, gitlab.InviteMemberOptions{
			Email:       request.Email,
			AccessLevel: request.AccessLevel,
			ExpiresAt:   request.ExpiresAt,
		})
		if err != nil {
			return nil, mapError(err)
		}
		return member, nil

	case "gitlab/searchBlobs":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		page, err := client.SearchBlobs(
			ctx,
			request.ProjectID,
			request.Search,
			request.Page,
			request.PerPage,
		)
		if err != nil {
			return nil, mapError(err)
		}
		return page, nil

	case "gitlab/pipelineTestReport":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		report, err := client.PipelineTestReport(ctx, request.ProjectID, request.PipelineID)
		if err != nil {
			return nil, mapError(err)
		}
		return report, nil

	case "gitlab/jobArtifacts":
		client, rpcErr := s.session(request.ConnectionID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		artifact, err := client.JobArtifacts(ctx, request.ProjectID, request.JobID, 0)
		if err != nil {
			return nil, mapError(err)
		}
		return artifact, nil
	}

	return nil, &Error{
		Code:    -32601,
		Kind:    "method_not_found",
		Message: "method not found: " + method,
	}
}

// requireWritable 是写操作统一闸门，M2 起被写方法调用。
func (s *Service) requireWritable(client *gitlab.Client) *Error {
	if client.ReadOnly() {
		return &Error{Code: -32003, Kind: gitlab.ErrReadOnly, Message: "read_only: connection is in read-only mode"}
	}
	return nil
}

func (s *Service) session(connectionID string) (*gitlab.Client, *Error) {
	if strings.TrimSpace(connectionID) == "" {
		return nil, invalidParams("missing connectionId")
	}
	s.mu.RLock()
	client := s.sessions[connectionID]
	s.mu.RUnlock()
	if client == nil {
		return nil, &Error{
			Code:    -32000,
			Kind:    "not_connected",
			Message: "connection is not established; reconnect the GitLab connection",
		}
	}
	return client, nil
}

func buildClient(request LifecycleRequest) (*gitlab.Client, *Error) {
	host := strings.TrimSpace(request.Runtime.Host)
	if host == "" {
		host = strings.TrimSpace(request.Connection.Host)
	}
	port := request.Runtime.Port
	if port == 0 {
		port = request.Connection.Port
	}
	if host == "" {
		return nil, invalidConfig("GitLab host is missing; check the connection settings")
	}

	externalConfig := request.Connection.ExternalConfig
	if len(externalConfig) == 0 {
		// 兼容个别宿主版本下发 camelCase 的情况。
		externalConfig = request.Connection.ExternalConfigNew
	}

	scheme := configString(externalConfig, "scheme", "https")
	basePath := configString(externalConfig, "base_path", "")
	baseURL, err := gitlab.ParseBaseURL(scheme, host, port, basePath)
	if err != nil {
		return nil, mapError(err)
	}

	timeoutSeconds := configInt(externalConfig, "connect_timeout_secs", 15)
	if timeoutSeconds < 1 {
		timeoutSeconds = 1
	}
	if timeoutSeconds > 300 {
		timeoutSeconds = 300
	}

	clientConfig := gitlab.Config{
		BaseURL:            baseURL,
		ReadOnly:           configBool(externalConfig, "read_only", request.Connection.ReadOnly),
		InsecureSkipVerify: configBool(externalConfig, "insecure_skip_verify", false),
		Timeout:            time.Duration(timeoutSeconds) * time.Second,
	}

	switch configString(externalConfig, "auth_method", "token") {
	case "password":
		clientConfig.Username = firstNonEmpty(
			request.Connection.Username,
			configString(externalConfig, "username", ""),
		)
		clientConfig.Password = connectionPassword(request)
		clientConfig.ClientID = configString(externalConfig, "oauth_client_id", "")
		clientConfig.ClientSecret = connectionSecret(request, "oauth_client_secret")
	default:
		clientConfig.Token = connectionToken(request)
	}

	client, err := gitlab.NewClient(clientConfig)
	if err != nil {
		return nil, mapError(err)
	}
	return client, nil
}

// connectionToken 只取访问令牌，不把密码当令牌使用。
func connectionToken(request LifecycleRequest) string {
	if token := stringValue(request.Connection.APIToken); token != "" {
		return token
	}
	for _, secrets := range []map[string]any{
		request.Connection.Secrets,
		request.Connection.ConnectionSecrets,
		request.Connection.ConnectionSecretsNew,
		request.Secrets,
	} {
		for _, key := range []string{"token", "api_token"} {
			if token := stringValue(secrets[key]); token != "" {
				return token
			}
		}
	}
	return ""
}

// connectionPassword 取账号密码认证的密码。
func connectionPassword(request LifecycleRequest) string {
	if password := stringValue(request.Connection.Password); password != "" {
		return password
	}
	return connectionSecret(request, "password")
}

// connectionSecret 在 DBX 各版本的密钥投递位置里查找指定键。
func connectionSecret(request LifecycleRequest, key string) string {
	for _, secrets := range []map[string]any{
		request.Connection.Secrets,
		request.Connection.ConnectionSecrets,
		request.Connection.ConnectionSecretsNew,
		request.Secrets,
	} {
		if value := stringValue(secrets[key]); value != "" {
			return value
		}
	}
	return ""
}

func connectionResult(client *gitlab.Client, info *gitlab.ConnectionInfo) map[string]any {
	message := "Connected to GitLab"
	if info.Version != "" {
		message = "Connected to GitLab " + info.Version
	}
	return map[string]any{
		"success":  true,
		"message":  message,
		"readOnly": client.ReadOnly(),
		"version":  info.Version,
		"user":     info.User,
	}
}

func configString(values map[string]any, key, fallback string) string {
	if values == nil {
		return fallback
	}
	if value, ok := values[key].(string); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func configBool(values map[string]any, key string, fallback bool) bool {
	if values == nil {
		return fallback
	}
	switch value := values[key].(type) {
	case bool:
		return value
	case string:
		if parsed, err := strconv.ParseBool(strings.TrimSpace(value)); err == nil {
			return parsed
		}
	case float64:
		return value != 0
	}
	return fallback
}

func configInt(values map[string]any, key string, fallback int) int {
	if values == nil {
		return fallback
	}
	switch value := values[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	return fallback
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func invalidParams(message string) *Error {
	return &Error{Code: -32602, Kind: "invalid_params", Message: message}
}

func invalidConfig(message string) *Error {
	return &Error{Code: -32602, Kind: gitlab.ErrInvalidConfig, Message: message}
}

func mapError(err error) *Error {
	var apiErr *gitlab.APIError
	if errors.As(err, &apiErr) {
		code := -32000
		switch apiErr.Kind {
		case gitlab.ErrInvalidConfig:
			code = -32602
		case gitlab.ErrUnauthorized:
			code = -32001
		case gitlab.ErrForbidden:
			code = -32003
		case gitlab.ErrNotFound:
			code = -32004
		}
		return &Error{Code: code, Kind: apiErr.Kind, Message: apiErr.Error()}
	}
	return &Error{Code: -32603, Kind: "server_error", Message: err.Error()}
}
