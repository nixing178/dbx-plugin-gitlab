package rpc

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/nixing178/dbx-plugin-gitlab/internal/gitlab"
)

// MCPTool 是暴露给 DBX 内置 AI / 外部 MCP 的工具定义。
type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

// MCPToolList 是 mcp/tools 的返回。
type MCPToolList struct {
	Tools []MCPTool `json:"tools"`
}

// MCPContent 是 CallToolResult 的内容块。
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPCallResult 是 mcp/call 的返回。
type MCPCallResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError"`
}

// writeTools 标记会修改 GitLab 数据的工具；只读连接下不暴露、也不执行。
var writeTools = map[string]bool{
	"gitlab_comment_mr":         true,
	"gitlab_reply_discussion":   true,
	"gitlab_resolve_discussion": true,
	"gitlab_approve_mr":         true,
	"gitlab_unapprove_mr":       true,
	"gitlab_merge_mr":           true,
	"gitlab_set_mr_state":       true,
	"gitlab_create_mr":          true,
	"gitlab_update_mr":          true,
	"gitlab_retry_job":          true,
	"gitlab_cancel_job":         true,
	"gitlab_play_job":           true,
	"gitlab_retry_pipeline":     true,
	"gitlab_cancel_pipeline":    true,
	"gitlab_create_pipeline":    true,
	"gitlab_create_issue":       true,
	"gitlab_comment_issue":      true,
	"gitlab_mark_todo_done":     true,
}

func (s *Service) handleMCPTools(raw json.RawMessage) (any, *Error) {
	var request struct {
		ConnectionID string `json:"connectionId"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, invalidParams("invalid mcp/tools parameters")
		}
	}
	client, rpcErr := s.session(request.ConnectionID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return MCPToolList{Tools: mcpTools(client.ReadOnly())}, nil
}

func (s *Service) handleMCPCall(ctx context.Context, raw json.RawMessage) (any, *Error) {
	var request struct {
		Tool         string          `json:"tool"`
		Arguments    map[string]any  `json:"arguments"`
		Lifecycle    json.RawMessage `json:"lifecycle"`
		ConnectionID string          `json:"connectionId"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, invalidParams("invalid mcp/call parameters")
	}
	if strings.TrimSpace(request.Tool) == "" {
		return nil, invalidParams("missing tool name")
	}

	client, temporary, rpcErr := s.mcpClient(request.ConnectionID, request.Lifecycle)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if temporary {
		defer client.CloseIdleConnections()
	}
	return callTool(ctx, client, request.Tool, request.Arguments), nil
}

// mcpClient 优先使用调用方传来的连接生命周期，否则复用已建立的会话。
func (s *Service) mcpClient(connectionID string, lifecycle json.RawMessage) (*gitlab.Client, bool, *Error) {
	if len(lifecycle) > 0 && string(lifecycle) != "null" {
		var request LifecycleRequest
		if err := json.Unmarshal(lifecycle, &request); err != nil {
			return nil, false, invalidParams("invalid lifecycle payload")
		}
		client, rpcErr := buildClient(request)
		if rpcErr != nil {
			return nil, false, rpcErr
		}
		return client, true, nil
	}

	client, rpcErr := s.session(connectionID)
	if rpcErr != nil {
		return nil, false, rpcErr
	}
	return client, false, nil
}

// mcpTools 返回当前连接可用的工具清单。只读连接不暴露写工具。
func mcpTools(readOnly bool) []MCPTool {
	projectID := map[string]any{
		"type":        "string",
		"description": "Project ID or full path, e.g. group/project",
	}
	page := map[string]any{"type": "integer", "description": "Page number, starting at 1"}
	perPage := map[string]any{"type": "integer", "description": "Items per page, max 100"}

	tools := []MCPTool{
		newTool(
			"gitlab_list_projects",
			"List GitLab projects the current user is a member of, newest activity first.",
			map[string]any{
				"search":  map[string]any{"type": "string", "description": "Filter by name or path"},
				"page":    page,
				"perPage": perPage,
			},
		),
		newTool(
			"gitlab_get_project",
			"Get details of a single GitLab project.",
			map[string]any{"projectId": projectID},
			"projectId",
		),
		newTool(
			"gitlab_list_files",
			"List files and directories of a repository path.",
			map[string]any{
				"projectId": projectID,
				"path":      map[string]any{"type": "string", "description": "Directory path, empty for the repository root"},
				"ref":       map[string]any{"type": "string", "description": "Branch, tag or commit SHA"},
				"page":      page,
				"perPage":   perPage,
			},
			"projectId",
		),
		newTool(
			"gitlab_get_file",
			"Read the raw content of a repository file.",
			map[string]any{
				"projectId": projectID,
				"path":      map[string]any{"type": "string", "description": "File path inside the repository"},
				"ref":       map[string]any{"type": "string", "description": "Branch, tag or commit SHA"},
			},
			"projectId", "path",
		),
		newTool(
			"gitlab_list_commits",
			"List commits of a branch, tag or the default branch.",
			map[string]any{
				"projectId": projectID,
				"ref":       map[string]any{"type": "string", "description": "Branch, tag or commit SHA"},
				"page":      page,
				"perPage":   perPage,
			},
			"projectId",
		),
		newTool(
			"gitlab_list_merge_requests",
			"List merge requests. Omit projectId to query across all projects.",
			map[string]any{
				"projectId": projectID,
				"scope": map[string]any{
					"type":        "string",
					"description": "Global scope: assigned_to_me, created_by_me or reviewer",
				},
				"state":   map[string]any{"type": "string", "description": "opened, closed, merged or all"},
				"search":  map[string]any{"type": "string", "description": "Filter by title or description"},
				"page":    page,
				"perPage": perPage,
			},
		),
		newTool(
			"gitlab_get_merge_request",
			"Get details of a merge request, including reviewers and pipeline status.",
			map[string]any{
				"projectId": projectID,
				"iid":       map[string]any{"type": "integer", "description": "Project-scoped merge request IID"},
			},
			"projectId", "iid",
		),
		newTool(
			"gitlab_get_mr_diffs",
			"Get file diffs of a merge request.",
			map[string]any{
				"projectId": projectID,
				"iid":       map[string]any{"type": "integer", "description": "Project-scoped merge request IID"},
				"page":      page,
				"perPage":   perPage,
			},
			"projectId", "iid",
		),
		newTool(
			"gitlab_list_pipelines",
			"List pipelines of a project, newest first.",
			map[string]any{
				"projectId": projectID,
				"ref":       map[string]any{"type": "string", "description": "Branch or tag"},
				"status": map[string]any{
					"type":        "string",
					"description": "created, preparing, pending, running, success, failed, canceled, skipped, manual",
				},
				"page":    page,
				"perPage": perPage,
			},
			"projectId",
		),
		newTool(
			"gitlab_get_pipeline_jobs",
			"List jobs of a pipeline with stage, status and duration.",
			map[string]any{
				"projectId":  projectID,
				"pipelineId": map[string]any{"type": "integer", "description": "Pipeline id"},
				"page":       page,
				"perPage":    perPage,
			},
			"projectId", "pipelineId",
		),
		newTool(
			"gitlab_get_job_log",
			"Read a chunk of a job log. Pass offset to fetch only new content.",
			map[string]any{
				"projectId": projectID,
				"jobId":     map[string]any{"type": "integer", "description": "Job id"},
				"offset":    map[string]any{"type": "integer", "description": "Bytes already read; 0 for the first chunk"},
			},
			"projectId", "jobId",
		),
		newTool(
			"gitlab_list_issues",
			"List issues. Omit projectId to query across all projects.",
			map[string]any{
				"projectId": projectID,
				"scope": map[string]any{
					"type":        "string",
					"description": "Global scope: assigned_to_me or created_by_me",
				},
				"state":   map[string]any{"type": "string", "description": "opened, closed or all"},
				"search":  map[string]any{"type": "string", "description": "Filter by title or description"},
				"page":    page,
				"perPage": perPage,
			},
		),
		newTool(
			"gitlab_get_issue",
			"Get details of an issue.",
			map[string]any{
				"projectId": projectID,
				"iid":       map[string]any{"type": "integer", "description": "Project-scoped issue IID"},
			},
			"projectId", "iid",
		),
		newTool(
			"gitlab_list_todos",
			"List your pending GitLab todos.",
			map[string]any{
				"state":   map[string]any{"type": "string", "description": "pending or done"},
				"page":    page,
				"perPage": perPage,
			},
		),
		newTool(
			"gitlab_list_members",
			"List project members including inherited group members.",
			map[string]any{
				"projectId": projectID,
				"search":    map[string]any{"type": "string", "description": "Filter by name or username"},
				"page":      page,
				"perPage":   perPage,
			},
			"projectId",
		),
		newTool(
			"gitlab_search_code",
			"Search code inside a project's default branch.",
			map[string]any{
				"projectId": projectID,
				"search":    map[string]any{"type": "string", "description": "Search term"},
				"page":      page,
				"perPage":   perPage,
			},
			"projectId", "search",
		),
		newTool(
			"gitlab_get_test_report",
			"Get the test report of a pipeline with failed test cases.",
			map[string]any{
				"projectId":  projectID,
				"pipelineId": map[string]any{"type": "integer", "description": "Pipeline id"},
			},
			"projectId", "pipelineId",
		),
	}

	if readOnly {
		return tools
	}
	return append(tools, mcpWriteTools(projectID)...)
}

// mcpWriteTools 是只在可写连接下暴露的工具。
func mcpWriteTools(projectID map[string]any) []MCPTool {
	iid := map[string]any{"type": "integer", "description": "Project-scoped merge request IID"}
	pipelineID := map[string]any{"type": "integer", "description": "Pipeline id"}
	jobID := map[string]any{"type": "integer", "description": "Job id"}
	discussionID := map[string]any{"type": "string", "description": "Discussion thread id"}
	body := map[string]any{"type": "string", "description": "Comment body in Markdown"}

	return []MCPTool{
		newTool(
			"gitlab_comment_mr",
			"Add a comment to a merge request. Pass position for an inline comment.",
			map[string]any{
				"projectId": projectID,
				"iid":       iid,
				"body":      body,
				"position": map[string]any{
					"type":        "object",
					"description": "Inline anchor: base_sha, head_sha, start_sha, new_path, old_path, position_type, new_line or old_line",
				},
			},
			"projectId", "iid", "body",
		),
		newTool(
			"gitlab_reply_discussion",
			"Reply inside an existing discussion thread.",
			map[string]any{
				"projectId":    projectID,
				"iid":          iid,
				"discussionId": discussionID,
				"body":         body,
			},
			"projectId", "iid", "discussionId", "body",
		),
		newTool(
			"gitlab_resolve_discussion",
			"Mark a discussion thread as resolved or unresolved.",
			map[string]any{
				"projectId":    projectID,
				"iid":          iid,
				"discussionId": discussionID,
				"resolved":     map[string]any{"type": "boolean", "description": "true to resolve, false to reopen"},
			},
			"projectId", "iid", "discussionId", "resolved",
		),
		newTool(
			"gitlab_approve_mr",
			"Approve a merge request.",
			map[string]any{
				"projectId": projectID,
				"iid":       iid,
				"sha":       map[string]any{"type": "string", "description": "Optional commit SHA to approve"},
			},
			"projectId", "iid",
		),
		newTool(
			"gitlab_unapprove_mr",
			"Revoke your approval on a merge request.",
			map[string]any{"projectId": projectID, "iid": iid},
			"projectId", "iid",
		),
		newTool(
			"gitlab_merge_mr",
			"Merge a merge request.",
			map[string]any{
				"projectId":          projectID,
				"iid":                iid,
				"sha":                map[string]any{"type": "string", "description": "Optional commit SHA to merge"},
				"squash":             map[string]any{"type": "boolean", "description": "Squash commits on merge"},
				"removeSourceBranch": map[string]any{"type": "boolean", "description": "Delete the source branch after merge"},
				"mergeCommitMessage": map[string]any{"type": "string", "description": "Custom merge commit message"},
			},
			"projectId", "iid",
		),
		newTool(
			"gitlab_set_mr_state",
			"Close or reopen a merge request.",
			map[string]any{
				"projectId":  projectID,
				"iid":        iid,
				"stateEvent": map[string]any{"type": "string", "description": "close or reopen"},
			},
			"projectId", "iid", "stateEvent",
		),
		newTool(
			"gitlab_create_mr",
			"Create a merge request from a source branch into a target branch.",
			map[string]any{
				"projectId":          projectID,
				"sourceBranch":       map[string]any{"type": "string", "description": "Source branch"},
				"targetBranch":       map[string]any{"type": "string", "description": "Target branch"},
				"title":              map[string]any{"type": "string", "description": "Merge request title"},
				"description":        map[string]any{"type": "string", "description": "Markdown description"},
				"squash":             map[string]any{"type": "boolean", "description": "Squash commits on merge"},
				"removeSourceBranch": map[string]any{"type": "boolean", "description": "Delete the source branch after merge"},
			},
			"projectId", "sourceBranch", "targetBranch", "title",
		),
		newTool(
			"gitlab_update_mr",
			"Update the title or description of a merge request.",
			map[string]any{
				"projectId":   projectID,
				"iid":         iid,
				"title":       map[string]any{"type": "string", "description": "New title"},
				"description": map[string]any{"type": "string", "description": "New Markdown description"},
			},
			"projectId", "iid",
		),
		newTool(
			"gitlab_retry_job",
			"Retry a job.",
			map[string]any{"projectId": projectID, "jobId": jobID},
			"projectId", "jobId",
		),
		newTool(
			"gitlab_cancel_job",
			"Cancel a running job.",
			map[string]any{"projectId": projectID, "jobId": jobID},
			"projectId", "jobId",
		),
		newTool(
			"gitlab_play_job",
			"Run a manual job.",
			map[string]any{"projectId": projectID, "jobId": jobID},
			"projectId", "jobId",
		),
		newTool(
			"gitlab_retry_pipeline",
			"Retry the failed jobs of a pipeline.",
			map[string]any{"projectId": projectID, "pipelineId": pipelineID},
			"projectId", "pipelineId",
		),
		newTool(
			"gitlab_cancel_pipeline",
			"Cancel a running pipeline.",
			map[string]any{"projectId": projectID, "pipelineId": pipelineID},
			"projectId", "pipelineId",
		),
		newTool(
			"gitlab_create_pipeline",
			"Trigger a new pipeline for a ref, optionally with variables.",
			map[string]any{
				"projectId": projectID,
				"ref":       map[string]any{"type": "string", "description": "Branch or tag"},
				"variables": map[string]any{
					"type":        "array",
					"description": "Pipeline variables as {\"key\":\"K\",\"value\":\"V\"} objects",
				},
			},
			"projectId", "ref",
		),
		newTool(
			"gitlab_create_issue",
			"Create an issue in a project.",
			map[string]any{
				"projectId":   projectID,
				"title":       map[string]any{"type": "string", "description": "Issue title"},
				"description": map[string]any{"type": "string", "description": "Markdown description"},
				"labels": map[string]any{
					"type":        "array",
					"description": "Label names",
				},
			},
			"projectId", "title",
		),
		newTool(
			"gitlab_comment_issue",
			"Add a comment to an issue.",
			map[string]any{
				"projectId": projectID,
				"iid":       iid,
				"body":      body,
			},
			"projectId", "iid", "body",
		),
		newTool(
			"gitlab_mark_todo_done",
			"Mark a GitLab todo as done.",
			map[string]any{
				"todoId": map[string]any{"type": "integer", "description": "Todo id"},
			},
			"todoId",
		),
	}
}

func newTool(name, description string, properties map[string]any, required ...string) MCPTool {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return MCPTool{Name: name, Description: description, InputSchema: schema}
}

func callTool(
	ctx context.Context,
	client *gitlab.Client,
	tool string,
	arguments map[string]any,
) *MCPCallResult {
	if writeTools[tool] && client.ReadOnly() {
		return mcpError("read_only: connection is in read-only mode")
	}

	switch tool {
	case "gitlab_list_projects":
		page, err := client.Projects(
			ctx,
			gitlab.ProjectListOptions{
				Search:  argString(arguments, "search"),
				Page:    argInt(arguments, "page", 1),
				PerPage: argInt(arguments, "perPage", 20),
			},
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)

	case "gitlab_get_project":
		project, err := client.Project(ctx, argString(arguments, "projectId"))
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(project)

	case "gitlab_list_files":
		page, err := client.Tree(
			ctx,
			argString(arguments, "projectId"),
			argString(arguments, "path"),
			argString(arguments, "ref"),
			argInt(arguments, "page", 1),
			argInt(arguments, "perPage", 50),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)

	case "gitlab_get_file":
		file, err := client.File(
			ctx,
			argString(arguments, "projectId"),
			argString(arguments, "path"),
			argString(arguments, "ref"),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(file)

	case "gitlab_list_commits":
		page, err := client.Commits(
			ctx,
			argString(arguments, "projectId"),
			argString(arguments, "ref"),
			argInt(arguments, "page", 1),
			argInt(arguments, "perPage", 20),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)

	case "gitlab_list_merge_requests":
		page, err := client.MergeRequests(
			ctx,
			argString(arguments, "projectId"),
			gitlab.MROptions{
				Scope:   argString(arguments, "scope"),
				State:   argString(arguments, "state"),
				Search:  argString(arguments, "search"),
				Page:    argInt(arguments, "page", 1),
				PerPage: argInt(arguments, "perPage", 20),
			},
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)

	case "gitlab_get_merge_request":
		mr, err := client.MergeRequest(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(mr)

	case "gitlab_get_mr_diffs":
		page, err := client.MRDiffs(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
			argInt(arguments, "page", 1),
			argInt(arguments, "perPage", 20),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)
	case "gitlab_comment_mr":
		position, err := gitlab.DecodePosition(argAny(arguments, "position"))
		if err != nil {
			return mcpErrorFrom(err)
		}
		note, err := client.CommentMR(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
			argString(arguments, "body"),
			position,
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(note)

	case "gitlab_reply_discussion":
		note, err := client.ReplyDiscussion(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
			argString(arguments, "discussionId"),
			argString(arguments, "body"),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(note)

	case "gitlab_resolve_discussion":
		discussion, err := client.ResolveDiscussion(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
			argString(arguments, "discussionId"),
			argBool(arguments, "resolved", true),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(discussion)

	case "gitlab_approve_mr":
		approval, err := client.ApproveMR(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
			argString(arguments, "sha"),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(approval)

	case "gitlab_unapprove_mr":
		if err := client.UnapproveMR(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
		); err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(map[string]any{"success": true})

	case "gitlab_merge_mr":
		mr, err := client.MergeMR(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
			gitlab.MergeOptions{
				SHA:                 argString(arguments, "sha"),
				Squash:              argBool(arguments, "squash", false),
				RemoveSourceBranch:  argBool(arguments, "removeSourceBranch", false),
				MergeCommitMessage:  argString(arguments, "mergeCommitMessage"),
				SquashCommitMessage: argString(arguments, "squashCommitMessage"),
			},
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(mr)

	case "gitlab_set_mr_state":
		mr, err := client.SetMRState(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
			argString(arguments, "stateEvent"),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(mr)

	case "gitlab_create_mr":
		mr, err := client.CreateMR(
			ctx,
			argString(arguments, "projectId"),
			gitlab.CreateMRInput{
				SourceBranch:       argString(arguments, "sourceBranch"),
				TargetBranch:       argString(arguments, "targetBranch"),
				Title:              argString(arguments, "title"),
				Description:        argString(arguments, "description"),
				RemoveSourceBranch: argBool(arguments, "removeSourceBranch", false),
				Squash:             argBool(arguments, "squash", false),
			},
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(mr)

	case "gitlab_update_mr":
		input := gitlab.UpdateMRInput{}
		if title := argString(arguments, "title"); title != "" {
			input.Title = &title
		}
		if description, ok := arguments["description"].(string); ok {
			input.Description = &description
		}
		if input.Title == nil && input.Description == nil {
			return mcpError("nothing to update: provide title or description")
		}
		mr, err := client.UpdateMR(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
			input,
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(mr)

	case "gitlab_list_pipelines":
		page, err := client.Pipelines(
			ctx,
			argString(arguments, "projectId"),
			gitlab.PipelineOptions{
				Ref:     argString(arguments, "ref"),
				Status:  argString(arguments, "status"),
				Page:    argInt(arguments, "page", 1),
				PerPage: argInt(arguments, "perPage", 20),
			},
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)

	case "gitlab_get_pipeline_jobs":
		page, err := client.PipelineJobs(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "pipelineId", 0),
			argInt(arguments, "page", 1),
			argInt(arguments, "perPage", 100),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)

	case "gitlab_get_job_log":
		trace, err := client.JobTrace(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "jobId", 0),
			int64(argInt(arguments, "offset", 0)),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(trace)

	case "gitlab_retry_job":
		job, err := client.RetryJob(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "jobId", 0),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(job)

	case "gitlab_cancel_job":
		job, err := client.CancelJob(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "jobId", 0),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(job)

	case "gitlab_play_job":
		job, err := client.PlayJob(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "jobId", 0),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(job)

	case "gitlab_retry_pipeline":
		pipeline, err := client.RetryPipeline(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "pipelineId", 0),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(pipeline)

	case "gitlab_cancel_pipeline":
		pipeline, err := client.CancelPipeline(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "pipelineId", 0),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(pipeline)

	case "gitlab_create_pipeline":
		var variables []gitlab.PipelineVariable
		if raw, ok := arguments["variables"].([]any); ok {
			for _, item := range raw {
				entry, isMap := item.(map[string]any)
				if !isMap {
					continue
				}
				variables = append(variables, gitlab.PipelineVariable{
					Key:   argString(entry, "key"),
					Value: argString(entry, "value"),
				})
			}
		}
		pipeline, err := client.CreatePipeline(
			ctx,
			argString(arguments, "projectId"),
			argString(arguments, "ref"),
			variables,
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(pipeline)

	case "gitlab_list_issues":
		page, err := client.Issues(ctx, gitlab.IssueOptions{
			ProjectID: argString(arguments, "projectId"),
			Scope:     argString(arguments, "scope"),
			State:     argString(arguments, "state"),
			Search:    argString(arguments, "search"),
			Page:      argInt(arguments, "page", 1),
			PerPage:   argInt(arguments, "perPage", 20),
		})
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)

	case "gitlab_get_issue":
		issue, err := client.Issue(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(issue)

	case "gitlab_list_todos":
		page, err := client.Todos(
			ctx,
			argString(arguments, "state"),
			argInt(arguments, "page", 1),
			argInt(arguments, "perPage", 20),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)

	case "gitlab_create_issue":
		var labels []string
		if raw, ok := arguments["labels"].([]any); ok {
			for _, item := range raw {
				if label, isString := item.(string); isString && strings.TrimSpace(label) != "" {
					labels = append(labels, label)
				}
			}
		}
		issue, err := client.CreateIssue(ctx, argString(arguments, "projectId"), gitlab.CreateIssueInput{
			Title:       argString(arguments, "title"),
			Description: argString(arguments, "description"),
			Labels:      labels,
		})
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(issue)

	case "gitlab_comment_issue":
		note, err := client.CommentIssue(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "iid", 0),
			argString(arguments, "body"),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(note)

	case "gitlab_mark_todo_done":
		todo, err := client.MarkTodoDone(ctx, argInt(arguments, "todoId", 0))
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(todo)

	case "gitlab_list_members":
		page, err := client.ProjectMembers(
			ctx,
			argString(arguments, "projectId"),
			argString(arguments, "search"),
			argInt(arguments, "page", 1),
			argInt(arguments, "perPage", 50),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)

	case "gitlab_search_code":
		page, err := client.SearchBlobs(
			ctx,
			argString(arguments, "projectId"),
			argString(arguments, "search"),
			argInt(arguments, "page", 1),
			argInt(arguments, "perPage", 20),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(page)

	case "gitlab_get_test_report":
		report, err := client.PipelineTestReport(
			ctx,
			argString(arguments, "projectId"),
			argInt(arguments, "pipelineId", 0),
		)
		if err != nil {
			return mcpErrorFrom(err)
		}
		return mcpJSON(report)
	}

	return mcpError("unknown tool: " + tool)
}

func mcpJSON(value any) *MCPCallResult {
	encoded, err := json.Marshal(value)
	if err != nil {
		return mcpError("cannot encode tool result: " + err.Error())
	}
	return &MCPCallResult{Content: []MCPContent{{Type: "text", Text: string(encoded)}}}
}

func mcpError(message string) *MCPCallResult {
	return &MCPCallResult{
		IsError: true,
		Content: []MCPContent{{Type: "text", Text: message}},
	}
}

func mcpErrorFrom(err error) *MCPCallResult { return mcpError(err.Error()) }

func argString(arguments map[string]any, key string) string {
	if arguments == nil {
		return ""
	}
	if value, ok := arguments[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func argInt(arguments map[string]any, key string, fallback int) int {
	if arguments == nil {
		return fallback
	}
	switch value := arguments[key].(type) {
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

func argAny(arguments map[string]any, key string) any {
	if arguments == nil {
		return nil
	}
	return arguments[key]
}

func argBool(arguments map[string]any, key string, fallback bool) bool {
	if arguments == nil {
		return fallback
	}
	switch value := arguments[key].(type) {
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
