package gitlab

import "time"

// User 是 GitLab 用户。
type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
	State     string `json:"state"`
}

// VersionInfo 是 /version 的返回。
type VersionInfo struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

// ConnectionInfo 是连接测试的结果。
type ConnectionInfo struct {
	User     *User  `json:"user"`
	Version  string `json:"version"`
	ReadOnly bool   `json:"read_only"`
}

// Namespace 是项目所属命名空间。
type Namespace struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	FullPath string `json:"full_path"`
	Kind     string `json:"kind"`
}

// ProjectStatistics 是 GitLab 项目统计；需要 statistics=true 且调用者有足够权限。
type ProjectStatistics struct {
	CommitCount    int   `json:"commit_count"`
	StorageSize    int64 `json:"storage_size"`
	RepositorySize int64 `json:"repository_size"`
	LFSObjectsSize int64 `json:"lfs_objects_size"`
	JobArtifacts   int64 `json:"job_artifacts_size"`
}

// Project 是 GitLab 项目。
type Project struct {
	ID                int                `json:"id"`
	Name              string             `json:"name"`
	Path              string             `json:"path"`
	PathWithNamespace string             `json:"path_with_namespace"`
	Description       string             `json:"description"`
	DefaultBranch     string             `json:"default_branch"`
	WebURL            string             `json:"web_url"`
	AvatarURL         string             `json:"avatar_url"`
	ReadmeURL         string             `json:"readme_url"`
	StarCount         int                `json:"star_count"`
	Starred           bool               `json:"starred"`
	ForksCount        int                `json:"forks_count"`
	OpenIssuesCount   int                `json:"open_issues_count"`
	Visibility        string             `json:"visibility"`
	Archived          bool               `json:"archived"`
	CreatedAt         time.Time          `json:"created_at"`
	LastActivityAt    time.Time          `json:"last_activity_at"`
	Namespace         Namespace          `json:"namespace"`
	Statistics        *ProjectStatistics `json:"statistics,omitempty"`
}

// ProjectPage 是项目分页结果。
type ProjectPage struct {
	Items   []Project `json:"items"`
	Page    int       `json:"page"`
	HasMore bool      `json:"has_more"`
	Total   int       `json:"total"`
}

// TreeNode 是仓库树节点。
type TreeNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Mode string `json:"mode"`
}

// TreePage 是目录分页结果。
type TreePage struct {
	Items   []TreeNode `json:"items"`
	Page    int        `json:"page"`
	HasMore bool       `json:"has_more"`
}

// FileContent 是文件原文。
type FileContent struct {
	Path     string `json:"path"`
	Ref      string `json:"ref"`
	Size     int64  `json:"size"`
	Text     string `json:"text"`
	TooLarge bool   `json:"too_large"`
	Binary   bool   `json:"binary"`
}

// Commit 是提交摘要。
type Commit struct {
	ID            string    `json:"id"`
	ShortID       string    `json:"short_id"`
	Title         string    `json:"title"`
	Message       string    `json:"message"`
	AuthorName    string    `json:"author_name"`
	AuthorEmail   string    `json:"author_email"`
	AuthoredDate  time.Time `json:"authored_date"`
	CommitterName string    `json:"committer_name"`
	CommittedDate time.Time `json:"committed_date"`
	WebURL        string    `json:"web_url"`
	ParentIDs     []string  `json:"parent_ids"`
}

// CommitPage 是提交分页结果。
// Total 来自 X-Total；实例不返回时为 0，UI 需自行回退到已加载条数。
type CommitPage struct {
	Items   []Commit `json:"items"`
	Page    int      `json:"page"`
	HasMore bool     `json:"has_more"`
	Total   int      `json:"total"`
}

// DiffFile 是单文件差异。
//
// 只保留 UI 真正用到的字段：a_mode / b_mode / collapsed / generated_file 在
// 不同 GitLab 版本里类型不一致（字符串 vs 数字、bool vs null），声明成强类型
// 会让整个 MR 的 diff 解析失败，而它们从未被读取。
type DiffFile struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
	TooLarge    bool   `json:"too_large"`
	Diff        string `json:"diff"`
}

// DiffRefs 是行内评论定位所需的三个 SHA。
type DiffRefs struct {
	BaseSHA  string `json:"base_sha"`
	HeadSHA  string `json:"head_sha"`
	StartSHA string `json:"start_sha"`
}

// Pipeline 是流水线摘要。
type Pipeline struct {
	ID        int       `json:"id"`
	IID       int       `json:"iid"`
	ProjectID int       `json:"project_id"`
	Status    string    `json:"status"`
	Ref       string    `json:"ref"`
	SHA       string    `json:"sha"`
	WebURL    string    `json:"web_url"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TimeStats 是合并请求的工时统计（估算与已花费）。
type TimeStats struct {
	TimeEstimate        int    `json:"time_estimate"`
	TotalTimeSpent      int    `json:"total_time_spent"`
	HumanTimeEstimate   string `json:"human_time_estimate"`
	HumanTotalTimeSpent string `json:"human_total_time_spent"`
}

// MR 是合并请求。
type MR struct {
	ID                  int        `json:"id"`
	IID                 int        `json:"iid"`
	ProjectID           int        `json:"project_id"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	State               string     `json:"state"`
	MergeStatus         string     `json:"merge_status"`
	DetailedMergeStatus string     `json:"detailed_merge_status"`
	HasConflicts        bool       `json:"has_conflicts"`
	Draft               bool       `json:"draft"`
	SourceBranch        string     `json:"source_branch"`
	TargetBranch        string     `json:"target_branch"`
	SHA                 string     `json:"sha"`
	WebURL              string     `json:"web_url"`
	Author              *User      `json:"author"`
	Assignees           []User     `json:"assignees"`
	Reviewers           []User     `json:"reviewers"`
	Labels              []string   `json:"labels"`
	UserNotesCount      int        `json:"user_notes_count"`
	ChangesCount        string     `json:"changes_count"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	MergedAt            *time.Time `json:"merged_at"`
	// DiffRefs 用于行内评论定位；/diffs 的数组响应不带它，详情里这份是权威值。
	DiffRefs            *DiffRefs  `json:"diff_refs"`
	Milestone           *Milestone `json:"milestone"`
	TimeStats           *TimeStats `json:"time_stats"`
	Pipeline            *Pipeline  `json:"pipeline"`
}

// MRPage 是合并请求分页结果。
type MRPage struct {
	Items   []MR `json:"items"`
	Page    int  `json:"page"`
	HasMore bool `json:"has_more"`
	Total   int  `json:"total"`
}

// MRStateCounts 是合并请求各状态的计数，用于页签上的数字。
// 某项为 -1 表示实例没有返回 X-Total（或该次查询失败），UI 只隐藏数字。
type MRStateCounts struct {
	Opened int `json:"opened"`
	Merged int `json:"merged"`
	Closed int `json:"closed"`
	All    int `json:"all"`
}

// MRDiffPage 是 MR 差异分页结果。
type MRDiffPage struct {
	Items    []DiffFile `json:"items"`
	Page     int        `json:"page"`
	HasMore  bool       `json:"has_more"`
	DiffRefs DiffRefs   `json:"diff_refs"`
}

// Position 是行内评论定位。
type Position struct {
	BaseSHA      string `json:"base_sha"`
	HeadSHA      string `json:"head_sha"`
	StartSHA     string `json:"start_sha"`
	NewPath      string `json:"new_path"`
	OldPath      string `json:"old_path"`
	PositionType string `json:"position_type"`
	NewLine      int    `json:"new_line"`
	OldLine      int    `json:"old_line"`
}

// Note 是评论。
type Note struct {
	ID         int       `json:"id"`
	Body       string    `json:"body"`
	Author     *User     `json:"author"`
	System     bool      `json:"system"`
	Resolvable bool      `json:"resolvable"`
	Resolved   bool      `json:"resolved"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Position   *Position `json:"position"`
}

// Discussion 是讨论线程。
type Discussion struct {
	ID             string `json:"id"`
	IndividualNote bool   `json:"individual_note"`
	Notes          []Note `json:"notes"`
}

// Approval 是合并请求的审批状态。
type Approval struct {
	ID          int       `json:"id"`
	IID         int       `json:"iid"`
	ProjectID   int       `json:"project_id"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	MergeStatus string    `json:"merge_status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MRVersion 是合并请求的一个变更版本（force-push 后会产生新版本）。
type MRVersion struct {
	ID             int       `json:"id"`
	HeadCommitSHA  string    `json:"head_commit_sha"`
	BaseCommitSHA  string    `json:"base_commit_sha"`
	StartCommitSHA string    `json:"start_commit_sha"`
	CreatedAt      time.Time `json:"created_at"`
	MergeRequestID int       `json:"merge_request_id"`
}

// Stage 是流水线阶段。
type Stage struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// DetailedStatus 是流水线的详细状态。
type DetailedStatus struct {
	Icon  string `json:"icon"`
	Text  string `json:"text"`
	Label string `json:"label"`
	Group string `json:"group"`
}

// PipelineDetail 是流水线详情，比列表多出阶段与耗时。
type PipelineDetail struct {
	Pipeline
	Stages         []Stage         `json:"stages"`
	Duration       int             `json:"duration"`
	QueuedDuration float64         `json:"queued_duration"`
	Coverage       string          `json:"coverage"`
	User           *User           `json:"user"`
	DetailedStatus *DetailedStatus `json:"detailed_status"`
}

// Job 是流水线作业。
type Job struct {
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	Stage          string     `json:"stage"`
	Status         string     `json:"status"`
	Ref            string     `json:"ref"`
	Tag            bool       `json:"tag"`
	AllowFailure   bool       `json:"allow_failure"`
	Duration       float64    `json:"duration"`
	QueuedDuration float64    `json:"queued_duration"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	WebURL         string     `json:"web_url"`
	User           *User      `json:"user"`
	Pipeline       *Pipeline  `json:"pipeline"`
	FailureReason  string     `json:"failure_reason"`
	ArtifactsFile  struct {
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	} `json:"artifacts_file"`
}

// JobTrace 是作业日志的一段增量内容。
type JobTrace struct {
	Text       string `json:"text"`
	NextOffset int64  `json:"next_offset"`
	Complete   bool   `json:"complete"`
	Size       int64  `json:"size"`
}

// PipelineVariable 是触发流水线时的变量。
type PipelineVariable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PipelinePage 是流水线分页结果。
type PipelinePage struct {
	Items   []Pipeline `json:"items"`
	Page    int        `json:"page"`
	HasMore bool       `json:"has_more"`
}

// JobPage 是作业分页结果。
type JobPage struct {
	Items   []Job `json:"items"`
	Page    int   `json:"page"`
	HasMore bool  `json:"has_more"`
}

// Milestone 是里程碑摘要。
type Milestone struct {
	ID      int    `json:"id"`
	IID     int    `json:"iid"`
	Title   string `json:"title"`
	State   string `json:"state"`
	DueDate string `json:"due_date"`
}

// Issue 是 GitLab Issue。
type Issue struct {
	ID             int        `json:"id"`
	IID            int        `json:"iid"`
	ProjectID      int        `json:"project_id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	State          string     `json:"state"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ClosedAt       *time.Time `json:"closed_at"`
	Author         *User      `json:"author"`
	Assignees      []User     `json:"assignees"`
	Labels         []string   `json:"labels"`
	Milestone      *Milestone `json:"milestone"`
	UserNotesCount int        `json:"user_notes_count"`
	WebURL         string     `json:"web_url"`
	References     struct {
		Short    string `json:"short"`
		Relative string `json:"relative"`
		Full     string `json:"full"`
	} `json:"references"`
	TaskCompletionStatus struct {
		Count          int `json:"count"`
		CompletedCount int `json:"completed_count"`
	} `json:"task_completion_status"`
}

// IssuePage 是 Issue 分页结果。
type IssuePage struct {
	Items   []Issue `json:"items"`
	Page    int     `json:"page"`
	HasMore bool    `json:"has_more"`
}

// TodoTarget 是待办指向的对象。
type TodoTarget struct {
	ID     int    `json:"id"`
	IID    int    `json:"iid"`
	Title  string `json:"title"`
	State  string `json:"state"`
	WebURL string `json:"web_url"`
}

// Todo 是 GitLab 待办项。
type Todo struct {
	ID         int         `json:"id"`
	ProjectID  int         `json:"project_id"`
	ActionName string      `json:"action_name"`
	TargetType string      `json:"target_type"`
	Target     *TodoTarget `json:"target"`
	TargetURL  string      `json:"target_url"`
	Body       string      `json:"body"`
	State      string      `json:"state"`
	CreatedAt  time.Time   `json:"created_at"`
	Author     *User       `json:"author"`
	Project    *Project    `json:"project"`
}

// TodoPage 是待办分页结果。
type TodoPage struct {
	Items   []Todo `json:"items"`
	Page    int    `json:"page"`
	HasMore bool   `json:"has_more"`
}

// Member 是项目成员。
type Member struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	State       string `json:"state"`
	AccessLevel int    `json:"access_level"`
	AvatarURL   string `json:"avatar_url"`
	WebURL      string `json:"web_url"`
	ExpiresAt   string `json:"expires_at"`
}

// MemberPage 是成员分页结果。
type MemberPage struct {
	Items   []Member `json:"items"`
	Page    int      `json:"page"`
	HasMore bool     `json:"has_more"`
}

// SearchResult 是代码搜索结果（scope=blobs）。
type SearchResult struct {
	Basename  string `json:"basename"`
	Filename  string `json:"filename"`
	Data      string `json:"data"`
	Path      string `json:"path"`
	Ref       string `json:"ref"`
	StartLine int    `json:"startline"`
	ProjectID int    `json:"project_id"`
}

// SearchPage 是搜索结果分页。
type SearchPage struct {
	Items   []SearchResult `json:"items"`
	Page    int            `json:"page"`
	HasMore bool           `json:"has_more"`
}

// TestCase 是单条测试用例。
type TestCase struct {
	Name          string  `json:"name"`
	Classname     string  `json:"classname"`
	File          string  `json:"file"`
	ExecutionTime float64 `json:"execution_time"`
	Status        string  `json:"status"`
	SystemOutput  string  `json:"system_output"`
	StackTrace    string  `json:"stack_trace"`
}

// TestSuite 是测试套件。
type TestSuite struct {
	Name         string     `json:"name"`
	TotalCount   int        `json:"total_count"`
	SuccessCount int        `json:"success_count"`
	FailedCount  int        `json:"failed_count"`
	SkippedCount int        `json:"skipped_count"`
	ErrorCount   int        `json:"error_count"`
	TotalTime    float64    `json:"total_time"`
	TestCases    []TestCase `json:"test_cases"`
}

// TestReport 是流水线测试报告。
type TestReport struct {
	TotalCount   int         `json:"total_count"`
	SuccessCount int         `json:"success_count"`
	FailedCount  int         `json:"failed_count"`
	SkippedCount int         `json:"skipped_count"`
	ErrorCount   int         `json:"error_count"`
	TotalTime    float64     `json:"total_time"`
	TestSuites   []TestSuite `json:"test_suites"`
}

// ArtifactDownload 是作业制品的下载结果；大文件只返回元信息。
type ArtifactDownload struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
	TooLarge bool   `json:"too_large"`
}
