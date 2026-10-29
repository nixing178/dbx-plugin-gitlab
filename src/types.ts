export interface GitLabUser {
  id: number;
  username: string;
  name: string;
  avatar_url: string;
  web_url: string;
  state?: string;
}

export interface ConnectionResult {
  success: boolean;
  message: string;
  readOnly: boolean;
  version: string;
  user: GitLabUser;
}

export interface WhoAmIResult {
  user: GitLabUser;
  read_only: boolean;
  version: string;
}

export interface GitLabNamespace {
  id: number;
  name: string;
  path: string;
  full_path: string;
  kind: string;
}

export interface GitLabProject {
  id: number;
  name: string;
  path: string;
  path_with_namespace: string;
  description: string;
  default_branch: string;
  web_url: string;
  avatar_url: string;
  readme_url: string;
  star_count: number;
  /** 当前用户是否已加星；部分实例的详情响应不带该字段。 */
  starred?: boolean;
  forks_count: number;
  open_issues_count: number;
  visibility: string;
  archived: boolean;
  created_at?: string;
  last_activity_at: string;
  namespace: GitLabNamespace;
  statistics?: ProjectStatistics | null;
}

/** 加星/取消星标的结果；star_count = -1 表示实例返回 304，本地计数保持不变。 */
export interface StarResult {
  starred: boolean;
  star_count: number;
}

export interface ProjectStatistics {
  commit_count: number;
  storage_size: number;
  repository_size: number;
  lfs_objects_size: number;
  job_artifacts_size: number;
}

export interface ProjectPage {
  items: GitLabProject[];
  page: number;
  has_more: boolean;
  total: number;
}

export interface TreeNode {
  id: string;
  name: string;
  type: "tree" | "blob";
  path: string;
  mode: string;
}

export interface TreePage {
  items: TreeNode[];
  page: number;
  has_more: boolean;
}

export interface FileContent {
  path: string;
  ref: string;
  size: number;
  text: string;
  too_large: boolean;
  binary: boolean;
}

export interface Commit {
  id: string;
  short_id: string;
  title: string;
  message: string;
  author_name: string;
  author_email: string;
  authored_date: string;
  committer_name: string;
  committed_date: string;
  web_url: string;
  parent_ids: string[];
}

export interface Branch {
  name: string;
  default: boolean;
  protected: boolean;
  commit?: Commit | null;
}

export interface BranchPage {
  items: Branch[];
  page: number;
  has_more: boolean;
}

/** 目录条目的「最后提交」。commit 为空表示该路径查询失败或无权限。 */
export interface PathCommit {
  path: string;
  commit: Commit | null;
}

/** 项目信息栏的聚合数据；计数为 -1 表示实例未返回总数。 */
export interface RepoOverview {
  branch_count: number;
  tag_count: number;
  commit_count: number;
  storage_size: number;
  repository_size: number;
  created_at: string;
  last_activity_at: string;
  head_commit: Commit | null;
}

export interface CommitPage {
  items: Commit[];
  page: number;
  has_more: boolean;
  /** 来自 X-Total；实例不返回时为 0。 */
  total: number;
}

/** 包含某个提交的分支或标签。 */
export interface CommitRef {
  type: string;
  name: string;
}

/** 提交详情页的聚合数据；refs / merge_requests 取不到时为空数组。 */
export interface CommitDetail {
  commit: Commit;
  refs: CommitRef[];
  merge_requests: MergeRequest[];
}

export interface DiffFile {
  old_path: string;
  new_path: string;
  new_file: boolean;
  renamed_file: boolean;
  deleted_file: boolean;
  too_large: boolean;
  diff: string;
  /** 下面几个字段后端不再解析（不同 GitLab 版本类型不一致），仅保留类型兼容。 */
  a_mode?: string;
  b_mode?: string;
  collapsed?: boolean;
  generated_file?: boolean;
}

export interface DiffRefs {
  base_sha: string;
  head_sha: string;
  start_sha: string;
}

export interface Pipeline {
  id: number;
  iid: number;
  project_id: number;
  status: string;
  ref: string;
  sha: string;
  web_url: string;
  source: string;
  created_at: string;
  updated_at: string;
}

export interface TimeStats {
  time_estimate: number;
  total_time_spent: number;
  human_time_estimate: string;
  human_total_time_spent: string;
}

export interface MergeRequest {
  id: number;
  iid: number;
  project_id: number;
  title: string;
  description: string;
  state: string;
  merge_status: string;
  detailed_merge_status: string;
  has_conflicts: boolean;
  draft: boolean;
  source_branch: string;
  target_branch: string;
  sha: string;
  web_url: string;
  author: GitLabUser | null;
  assignees: GitLabUser[];
  reviewers: GitLabUser[];
  labels: string[];
  user_notes_count: number;
  changes_count: string;
  created_at: string;
  updated_at: string;
  merged_at: string | null;
  /** /diffs 的数组响应不带 diff_refs，行内评论用详情里这份。 */
  diff_refs?: DiffRefs | null;
  milestone: Milestone | null;
  time_stats: TimeStats | null;
  pipeline: Pipeline | null;
}

/** GitLab 把审批人包在 user 字段里。 */
export interface MRApprovalVoter {
  user: GitLabUser | null;
}

export interface MRApprovals {
  id: number;
  iid: number;
  approvals_required: number;
  approvals_left: number;
  approved: boolean;
  approved_by: MRApprovalVoter[];
  suggested_approvers: GitLabUser[];
}

export interface AwardEmoji {
  id: number;
  name: string;
  user: GitLabUser | null;
  awardable_id: number;
  awardable_type: string;
}

export interface MRPage {
  items: MergeRequest[];
  page: number;
  has_more: boolean;
  /** 筛选条件下的总数；实例没返回 X-Total 时为 -1。 */
  total: number;
}

/** 分支比较里的单文件统计；diff 正文不进 UI，只留增删行数。 */
export interface CompareFile {
  old_path: string;
  new_path: string;
  new_file: boolean;
  renamed_file: boolean;
  deleted_file: boolean;
  too_large: boolean;
  additions: number;
  deletions: number;
}

/** 新建合并请求页「比较分支」的结果。 */
export interface CompareResult {
  commits: Commit[];
  diffs: CompareFile[];
  compare_timeout: boolean;
  compare_same_ref: boolean;
}

export interface Label {
  id: number;
  name: string;
  color: string;
  text_color: string;
  description: string;
}

export interface LabelPage {
  items: Label[];
  page: number;
  has_more: boolean;
}

export interface MilestonePage {
  items: Milestone[];
  page: number;
  has_more: boolean;
}

/** 合并请求页签计数；-1 表示实例没返回 X-Total。 */
export interface MrCounts {
  opened: number;
  merged: number;
  closed: number;
  all: number;
}

export interface MRDiffPage {
  items: DiffFile[];
  page: number;
  has_more: boolean;
  diff_refs: DiffRefs;
}

export interface NotePosition {
  base_sha: string;
  head_sha: string;
  start_sha: string;
  new_path: string;
  old_path: string;
  position_type: string;
  new_line: number;
  old_line: number;
}

export interface Note {
  id: number;
  body: string;
  author: GitLabUser | null;
  system: boolean;
  resolvable: boolean;
  resolved: boolean;
  created_at: string;
  updated_at: string;
  position: NotePosition | null;
}

export interface Discussion {
  id: string;
  individual_note: boolean;
  notes: Note[];
}

export interface Approval {
  id: number;
  iid: number;
  project_id: number;
  title: string;
  state: string;
  merge_status: string;
  created_at: string;
  updated_at: string;
}

export interface MRVersion {
  id: number;
  head_commit_sha: string;
  base_commit_sha: string;
  start_commit_sha: string;
  created_at: string;
  merge_request_id: number;
}

export interface CommentPosition {
  base_sha: string;
  head_sha: string;
  start_sha: string;
  new_path: string;
  old_path: string;
  position_type: "text";
  new_line?: number;
  old_line?: number;
}

export interface Stage {
  name: string;
  status: string;
}

export interface DetailedStatus {
  icon: string;
  text: string;
  label: string;
  group: string;
}

export interface PipelineDetail extends Pipeline {
  stages: Stage[];
  duration: number;
  queued_duration: number;
  coverage: string;
  user: GitLabUser | null;
  detailed_status: DetailedStatus | null;
}

export interface Job {
  id: number;
  name: string;
  stage: string;
  status: string;
  ref: string;
  tag: boolean;
  allow_failure: boolean;
  duration: number;
  queued_duration: number;
  created_at: string;
  started_at: string | null;
  finished_at: string | null;
  web_url: string;
  user: GitLabUser | null;
  pipeline: Pipeline | null;
  failure_reason: string;
  artifacts_file: { filename: string; size: number } | null;
}

export interface JobTrace {
  text: string;
  next_offset: number;
  complete: boolean;
  size: number;
}

export interface PipelinePage {
  items: Pipeline[];
  page: number;
  has_more: boolean;
}

export interface JobPage {
  items: Job[];
  page: number;
  has_more: boolean;
}

export interface PipelineVariable {
  key: string;
  value: string;
}

export interface Milestone {
  id: number;
  iid: number;
  title: string;
  state: string;
  due_date: string;
}

export interface Issue {
  id: number;
  iid: number;
  project_id: number;
  title: string;
  description: string;
  state: string;
  created_at: string;
  updated_at: string;
  closed_at: string | null;
  author: GitLabUser | null;
  assignees: GitLabUser[];
  labels: string[];
  milestone: Milestone | null;
  user_notes_count: number;
  web_url: string;
  references: { short: string; relative: string; full: string };
  task_completion_status: { count: number; completed_count: number };
}

export interface IssuePage {
  items: Issue[];
  page: number;
  has_more: boolean;
}

export interface TodoTarget {
  id: number;
  iid: number;
  title: string;
  state: string;
  web_url: string;
}

export interface Todo {
  id: number;
  project_id: number;
  action_name: string;
  target_type: string;
  target: TodoTarget | null;
  target_url: string;
  body: string;
  state: string;
  created_at: string;
  author: GitLabUser | null;
  project: GitLabProject | null;
}

export interface TodoPage {
  items: Todo[];
  page: number;
  has_more: boolean;
}

export interface Member {
  id: number;
  username: string;
  name: string;
  state: string;
  access_level: number;
  avatar_url: string;
  web_url: string;
  expires_at: string | null;
  /** 加入时间；老版本 GitLab 可能不返回。 */
  created_at?: string;
  /** direct / inherited；后端取不到直接成员清单时为空。 */
  source?: string;
}

export interface MemberPage {
  items: Member[];
  page: number;
  has_more: boolean;
}

export interface SearchResult {
  basename: string;
  filename: string;
  data: string;
  path: string;
  ref: string;
  startline: number;
  project_id: number;
}

export interface SearchPage {
  items: SearchResult[];
  page: number;
  has_more: boolean;
}

export interface TestCase {
  name: string;
  classname: string;
  file: string;
  execution_time: number;
  status: string;
  system_output: string;
  stack_trace: string;
}

export interface TestSuite {
  name: string;
  total_count: number;
  success_count: number;
  failed_count: number;
  skipped_count: number;
  error_count: number;
  total_time: number;
  test_cases: TestCase[];
}

export interface TestReport {
  total_count: number;
  success_count: number;
  failed_count: number;
  skipped_count: number;
  error_count: number;
  total_time: number;
  test_suites: TestSuite[];
}

export interface ArtifactDownload {
  filename: string;
  size: number;
  encoding: string;
  data: string;
  too_large: boolean;
}
