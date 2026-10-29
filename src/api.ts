import type {
  ArtifactDownload,
  Approval,
  AwardEmoji,
  BranchPage,
  CommitDetail,
  CommitPage,
  CommentPosition,
  CompareResult,
  ConnectionResult,
  DiffFile,
  Discussion,
  FileContent,
  GitLabProject,
  Issue,
  IssuePage,
  Job,
  JobPage,
  JobTrace,
  LabelPage,
  MemberPage,
  MergeRequest,
  MilestonePage,
  MRApprovals,
  MrCounts,
  MRDiffPage,
  MRPage,
  MRVersion,
  Note,
  PathCommit,
  Pipeline,
  PipelineDetail,
  PipelinePage,
  PipelineVariable,
  ProjectPage,
  RepoOverview,
  SearchPage,
  StarResult,
  TestReport,
  Todo,
  TodoPage,
  TreePage,
  WhoAmIResult,
} from "./types";

let currentConnectionId = "";

/**
 * 连接就绪闸门。
 * 子组件的 onMounted 早于 App 的异步 onMounted，若直接发请求会拿到空 connectionId，
 * sidecar 会回 "missing connectionId"。所有 RPC 统一等这里放行。
 */
let releaseConnection: (() => void) | null = null;
const connectionReady = new Promise<void>((resolve) => {
  releaseConnection = resolve;
});

function readHostConnectionId(): string {
  try {
    return window.dbxPlugin?.context?.connectionId ?? "";
  } catch {
    return "";
  }
}

export function setConnectionId(id: string | undefined): void {
  currentConnectionId = id ?? "";
  releaseConnection?.();
  releaseConnection = null;
}

/** 宿主可能在模块加载前就注入了 context，能拿到就直接放行。 */
const bootstrapId = readHostConnectionId();
if (bootstrapId) {
  setConnectionId(bootstrapId);
}

export function getConnectionId(): string {
  return currentConnectionId;
}

/**
 * 调用本插件 sidecar 的 RPC。
 * 宿主不会自动补 connectionId，必须由 UI 显式携带。
 */
export async function invoke<T>(
  method: string,
  params: Record<string, unknown> = {},
  timeoutMs = 30_000,
): Promise<T> {
  const plugin = window.dbxPlugin;
  if (!plugin) {
    throw new Error("DBX host bridge is not available.");
  }
  await connectionReady;
  return plugin.invoke<T>(
    method,
    { connectionId: currentConnectionId, ...params },
    { timeoutMs },
  );
}

export function describeError(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  try {
    return JSON.stringify(error);
  } catch {
    return "Unknown error";
  }
}

export const gitlabApi = {
  whoami(): Promise<WhoAmIResult> {
    return invoke<WhoAmIResult>("gitlab/whoami", {}, 15_000);
  },

  projects(
    params: {
      search?: string;
      page?: number;
      perPage?: number;
      starred?: boolean;
      owned?: boolean;
      orderBy?: string;
      sort?: string;
      lastActivityBefore?: string;
    } = {},
  ): Promise<ProjectPage> {
    return invoke<ProjectPage>("gitlab/projects", { ...params });
  },

  project(projectId: string): Promise<GitLabProject> {
    return invoke<GitLabProject>("gitlab/project", { projectId });
  },

  /** 当前用户是否已加星；详情接口不返回该字段，单独查一次。 */
  projectStarred(projectId: string): Promise<{ starred: boolean }> {
    return invoke<{ starred: boolean }>("gitlab/projectStarred", { projectId }, 10_000);
  },

  /** starred=true 加星，false 取消；重复操作由后端按 304 幂等处理。 */
  starProject(projectId: string, starred: boolean): Promise<StarResult> {
    return invoke<StarResult>("gitlab/starProject", { projectId, starred });
  },

  tree(
    projectId: string,
    path = "",
    ref = "",
    page = 1,
    perPage = 100,
  ): Promise<TreePage> {
    return invoke<TreePage>("gitlab/tree", { projectId, path, ref, page, perPage });
  },

  file(projectId: string, path: string, ref = ""): Promise<FileContent> {
    return invoke<FileContent>("gitlab/file", { projectId, path, ref });
  },

  branches(projectId: string, search = "", page = 1, perPage = 100): Promise<BranchPage> {
    return invoke<BranchPage>("gitlab/branches", { projectId, search, page, perPage });
  },

  compare(projectId: string, from: string, to: string): Promise<CompareResult> {
    return invoke<CompareResult>("gitlab/compare", { projectId, from, to }, 45_000);
  },

  labels(projectId: string, search = "", page = 1, perPage = 100): Promise<LabelPage> {
    return invoke<LabelPage>("gitlab/labels", { projectId, search, page, perPage });
  },

  milestones(
    projectId: string,
    state = "active",
    page = 1,
    perPage = 100,
  ): Promise<MilestonePage> {
    return invoke<MilestonePage>("gitlab/milestones", { projectId, state, page, perPage });
  },

  repoOverview(projectId: string, ref = ""): Promise<RepoOverview> {
    return invoke<RepoOverview>("gitlab/repoOverview", { projectId, ref }, 45_000);
  },

  async treeCommits(projectId: string, ref: string, paths: string[]): Promise<PathCommit[]> {
    const result = await invoke<{ items: PathCommit[] }>(
      "gitlab/treeCommits",
      { projectId, ref, paths },
      45_000,
    );
    return result.items ?? [];
  },

  commits(
    projectId: string,
    ref = "",
    page = 1,
    perPage = 30,
  ): Promise<CommitPage> {
    return invoke<CommitPage>("gitlab/commits", { projectId, ref, page, perPage });
  },

  async commitDiff(projectId: string, sha: string): Promise<DiffFile[]> {
    const result = await invoke<{ items: DiffFile[] }>("gitlab/commitDiff", { projectId, sha });
    return result.items ?? [];
  },

  commitDetail(projectId: string, sha: string): Promise<CommitDetail> {
    return invoke<CommitDetail>("gitlab/commitDetail", { projectId, sha }, 45_000);
  },

  mergeRequests(params: {
    projectId?: string;
    scope?: string;
    state?: string;
    search?: string;
    orderBy?: string;
    sort?: string;
    page?: number;
    perPage?: number;
  }): Promise<MRPage> {
    return invoke<MRPage>("gitlab/mrs", params);
  },

  mrCounts(params: {
    projectId?: string;
    scope?: string;
    search?: string;
  }): Promise<MrCounts> {
    return invoke<MrCounts>("gitlab/mrCounts", params, 20_000);
  },

  mergeRequest(projectId: string, iid: number): Promise<MergeRequest> {
    return invoke<MergeRequest>("gitlab/mr", { projectId, iid });
  },

  mrDiffs(
    projectId: string,
    iid: number,
    page = 1,
    perPage = 20,
  ): Promise<MRDiffPage> {
    // 后端对 diff 给了 90s 读取窗口，前端要比它长，否则会先超时。
    return invoke<MRDiffPage>("gitlab/mrDiffs", { projectId, iid, page, perPage }, 120_000);
  },

  /** MR 范围内的提交（不是整条源分支历史），total 来自 X-Total。 */
  mrCommits(
    projectId: string,
    iid: number,
    page = 1,
    perPage = 30,
  ): Promise<CommitPage> {
    return invoke<CommitPage>("gitlab/mrCommits", { projectId, iid, page, perPage });
  },

  async mrDiscussions(projectId: string, iid: number): Promise<Discussion[]> {
    const result = await invoke<{ items: Discussion[] }>("gitlab/mrDiscussions", {
      projectId,
      iid,
    });
    return result.items ?? [];
  },

  async mrVersions(projectId: string, iid: number): Promise<MRVersion[]> {
    const result = await invoke<{ items: MRVersion[] }>("gitlab/mrVersions", {
      projectId,
      iid,
    });
    return result.items ?? [];
  },

  commentMr(params: {
    projectId: string;
    iid: number;
    body: string;
    position?: CommentPosition;
  }): Promise<Note> {
    return invoke<Note>("gitlab/commentMr", { ...params }, 30_000);
  },

  replyDiscussion(params: {
    projectId: string;
    iid: number;
    discussionId: string;
    body: string;
  }): Promise<Note> {
    return invoke<Note>("gitlab/replyDiscussion", { ...params }, 30_000);
  },

  resolveDiscussion(params: {
    projectId: string;
    iid: number;
    discussionId: string;
    resolved: boolean;
  }): Promise<Discussion> {
    return invoke<Discussion>("gitlab/resolveDiscussion", { ...params }, 30_000);
  },

  approveMr(projectId: string, iid: number, sha = ""): Promise<Approval> {
    return invoke<Approval>("gitlab/approveMr", { projectId, iid, sha }, 30_000);
  },

  unapproveMr(projectId: string, iid: number): Promise<{ success: boolean }> {
    return invoke<{ success: boolean }>("gitlab/unapproveMr", { projectId, iid }, 30_000);
  },

  mrApprovals(projectId: string, iid: number): Promise<MRApprovals> {
    return invoke<MRApprovals>("gitlab/mrApprovals", { projectId, iid });
  },

  async mrAwardEmoji(projectId: string, iid: number): Promise<AwardEmoji[]> {
    const result = await invoke<{ items: AwardEmoji[] }>("gitlab/mrAwardEmoji", {
      projectId,
      iid,
    });
    return result.items ?? [];
  },

  addMrAwardEmoji(projectId: string, iid: number, emojiName: string): Promise<AwardEmoji> {
    return invoke<AwardEmoji>("gitlab/addMrAwardEmoji", { projectId, iid, emojiName }, 30_000);
  },

  removeMrAwardEmoji(
    projectId: string,
    iid: number,
    awardId: number,
  ): Promise<{ success: boolean }> {
    return invoke<{ success: boolean }>(
      "gitlab/removeMrAwardEmoji",
      { projectId, iid, awardId },
      30_000,
    );
  },

  mergeMr(params: {
    projectId: string;
    iid: number;
    sha?: string;
    squash?: boolean;
    removeSourceBranch?: boolean;
    mergeCommitMessage?: string;
  }): Promise<MergeRequest> {
    return invoke<MergeRequest>("gitlab/mergeMr", { ...params }, 60_000);
  },

  setMrState(projectId: string, iid: number, stateEvent: "close" | "reopen"): Promise<MergeRequest> {
    return invoke<MergeRequest>("gitlab/setMrState", { projectId, iid, stateEvent }, 30_000);
  },

  createMr(params: {
    projectId: string;
    sourceBranch: string;
    targetBranch: string;
    title: string;
    description?: string;
    squash?: boolean;
    removeSourceBranch?: boolean;
    labels?: string[];
    assigneeIds?: number[];
    reviewerIds?: number[];
    milestoneId?: number;
  }): Promise<MergeRequest> {
    return invoke<MergeRequest>("gitlab/createMr", { ...params }, 60_000);
  },

  updateMr(params: {
    projectId: string;
    iid: number;
    title?: string;
    description?: string;
    assigneeIds?: number[];
    reviewerIds?: number[];
    labels?: string[];
    milestoneId?: number;
  }): Promise<MergeRequest> {
    return invoke<MergeRequest>("gitlab/updateMr", { ...params }, 30_000);
  },

  pipelines(params: {
    projectId: string;
    ref?: string;
    status?: string;
    page?: number;
    perPage?: number;
  }): Promise<PipelinePage> {
    return invoke<PipelinePage>("gitlab/pipelines", { ...params });
  },

  pipeline(projectId: string, pipelineId: number): Promise<PipelineDetail> {
    return invoke<PipelineDetail>("gitlab/pipeline", { projectId, pipelineId });
  },

  pipelineJobs(
    projectId: string,
    pipelineId: number,
    page = 1,
    perPage = 100,
  ): Promise<JobPage> {
    return invoke<JobPage>("gitlab/pipelineJobs", {
      projectId,
      pipelineId,
      page,
      perPage,
    });
  },

  jobTrace(projectId: string, jobId: number, offset: number): Promise<JobTrace> {
    return invoke<JobTrace>("gitlab/jobTrace", { projectId, jobId, offset }, 30_000);
  },

  retryJob(projectId: string, jobId: number): Promise<Job> {
    return invoke<Job>("gitlab/retryJob", { projectId, jobId }, 30_000);
  },

  cancelJob(projectId: string, jobId: number): Promise<Job> {
    return invoke<Job>("gitlab/cancelJob", { projectId, jobId }, 30_000);
  },

  playJob(projectId: string, jobId: number): Promise<Job> {
    return invoke<Job>("gitlab/playJob", { projectId, jobId }, 30_000);
  },

  retryPipeline(projectId: string, pipelineId: number): Promise<Pipeline> {
    return invoke<Pipeline>("gitlab/retryPipeline", { projectId, pipelineId }, 30_000);
  },

  cancelPipeline(projectId: string, pipelineId: number): Promise<Pipeline> {
    return invoke<Pipeline>("gitlab/cancelPipeline", { projectId, pipelineId }, 30_000);
  },

  createPipeline(
    projectId: string,
    ref: string,
    variables: PipelineVariable[] = [],
  ): Promise<Pipeline> {
    return invoke<Pipeline>("gitlab/createPipeline", { projectId, ref, variables }, 60_000);
  },

  issues(params: {
    projectId?: string;
    scope?: string;
    state?: string;
    search?: string;
    labels?: string[];
    page?: number;
    perPage?: number;
  }): Promise<IssuePage> {
    return invoke<IssuePage>("gitlab/issues", { ...params });
  },

  issue(projectId: string, iid: number): Promise<Issue> {
    return invoke<Issue>("gitlab/issue", { projectId, iid });
  },

  async issueNotes(projectId: string, iid: number): Promise<Note[]> {
    const result = await invoke<{ items: Note[] }>("gitlab/issueNotes", {
      projectId,
      iid,
    });
    return result.items ?? [];
  },

  todos(state = "pending", page = 1, perPage = 20): Promise<TodoPage> {
    return invoke<TodoPage>("gitlab/todos", { state, page, perPage });
  },

  createIssue(params: {
    projectId: string;
    title: string;
    description?: string;
    labels?: string[];
    dueDate?: string;
  }): Promise<Issue> {
    return invoke<Issue>("gitlab/createIssue", { ...params }, 30_000);
  },

  updateIssue(params: {
    projectId: string;
    iid: number;
    title?: string;
    description?: string;
    stateEvent?: "close" | "reopen";
  }): Promise<Issue> {
    return invoke<Issue>("gitlab/updateIssue", { ...params }, 30_000);
  },

  commentIssue(projectId: string, iid: number, body: string): Promise<Note> {
    return invoke<Note>("gitlab/commentIssue", { projectId, iid, body }, 30_000);
  },

  markTodoDone(todoId: number): Promise<Todo> {
    return invoke<Todo>("gitlab/markTodoDone", { todoId }, 30_000);
  },

  projectMembers(
    projectId: string,
    search = "",
    page = 1,
    perPage = 50,
  ): Promise<MemberPage> {
    return invoke<MemberPage>("gitlab/projectMembers", {
      projectId,
      search,
      page,
      perPage,
    });
  },

  searchBlobs(
    projectId: string,
    search: string,
    page = 1,
    perPage = 20,
  ): Promise<SearchPage> {
    return invoke<SearchPage>("gitlab/searchBlobs", {
      projectId,
      search,
      page,
      perPage,
    });
  },

  pipelineTestReport(projectId: string, pipelineId: number): Promise<TestReport> {
    return invoke<TestReport>("gitlab/pipelineTestReport", { projectId, pipelineId });
  },

  jobArtifacts(projectId: string, jobId: number): Promise<ArtifactDownload> {
    return invoke<ArtifactDownload>("gitlab/jobArtifacts", { projectId, jobId }, 60_000);
  },

  testConnection(): Promise<ConnectionResult> {
    return invoke<ConnectionResult>("connection/test", {}, 20_000);
  },
};
