# GitLab Console for DBX

[![CI](https://github.com/nixing178/dbx-plugin-gitlab/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/nixing178/dbx-plugin-gitlab/actions/workflows/ci.yml)

在 DBX 工作台内浏览 GitLab 项目与代码、评审合并请求，并把同一套能力通过 MCP 工具交给内置 AI。

目标实例：GitLab 社区版 / 企业版 17.x（REST API v4）。

## 当前进度

| 里程碑 | 内容 | 状态 |
|---|---|---|
| M1 | 连接、项目与代码、MR 只读（列表/详情/diff/讨论）、MCP 项目与代码工具 | ✅ 已完成 |
| M2 | MR 写操作：评论、resolve、approve、merge、创建/编辑 | ✅ 已完成 |
| M3 | Pipeline 与 Job 日志实时 tail、重试/取消 | ✅ 已完成 |
| M4 | Issues / Todos、账号密码认证 | ✅ 已完成 |
| M5 | 制品与测试报告、成员、代码搜索 | ✅ 已完成 |
| UI | 布局与交互对齐 GitLab 网页（左栏导航 / 面包屑 / 三栏详情 / Diff 折叠与 Viewed / stage 看板 + Job 抽屉） | ✅ 已完成 |
| UI | 项目页对齐 GitLab 原生（标题行 / 分支工具行 / 提交条 / 文件三列表格 / 右侧项目信息） | ✅ 已完成 |
| UI | 提交页对齐 GitLab（日期分组 / 作者筛选 / 消息搜索 / sha 复制 / 整宽列表） | ✅ 已完成 |
| UI | 提交详情页（提交列表可点开：上级提交 / 分支 / 标签 / 关联 MR / 变更统计） | ✅ 已完成 |
| UI | 项目页「最新提交」条可点开提交详情（复制按钮不误触跳转） | ✅ 已完成 |
| UI | 合并请求列表对齐 GitLab（状态页签带计数 / 筛选与排序 / 空状态插画 / 列表行） | ✅ 已完成 |
| UI | 新建合并请求整页化（分支比较 / 首提交预填标题描述 / 提交与变更 / 指派人·审核者·里程碑·标记 / Markdown 描述） | ✅ 已完成 |
| UI | 提交入口全覆盖（文件表最后提交、流水线列表与详情 sha、MR 流水线 tab、新建 MR 分支卡片与提交行） | ✅ 已完成 |
| MCP | 外部 MCP 开放：35 个工具按 4 个工具集分级声明（read / write / full） | ✅ 已完成 |
| 后续 | 分支保护、制品树浏览、OAuth 2.0 | 待排期 |

### M1 已支持

- **连接**：HTTPS/HTTP、上下文路径、TLS 跳过校验、连接超时、只读开关、多实例
- **项目**：参与项目列表（按最近活跃排序）、搜索、分页
- **代码**：README 渲染、文件树、文件内容、提交历史、单次提交 diff
- **合并请求**：全局三视图（待我评审 / 我创建 / 分配给我）+ 项目内列表、MR 详情、逐文件 diff、讨论与行内评论展示
- **MCP 工具**：`gitlab_list_projects`、`gitlab_get_project`、`gitlab_list_files`、`gitlab_get_file`、`gitlab_list_commits`、`gitlab_list_merge_requests`、`gitlab_get_merge_request`、`gitlab_get_mr_diffs`

M1 全部为只读操作；Pipelines 与 Issues 页签在后续里程碑提供。

### M2 已支持（写操作）

- **评论**：MR 讨论区发评论、线程内回复、标记解决 / 重新打开
- **行内评论**：在 diff 行上点 `+`，携带 `diff_refs` 精确定位到文件与行
- **审批**：Approve / 撤销审批（默认二次确认）
- **合并**：可选 squash、合并后删除源分支；关闭 / 重新打开 MR
- **创建与编辑**：新建 MR（源/目标分支、标题、描述）、编辑标题与描述
- **安全**：所有写操作经 RPC 层 `requireWritable` 闸门；只读连接下入口禁用、MCP 写工具不暴露
- **MCP 写工具**：`gitlab_comment_mr`、`gitlab_reply_discussion`、`gitlab_resolve_discussion`、`gitlab_approve_mr`、`gitlab_unapprove_mr`、`gitlab_merge_mr`、`gitlab_set_mr_state`、`gitlab_create_mr`、`gitlab_update_mr`

> 执行写操作前，需在连接配置里关闭「只读模式」；命令行与 AI 调用同样受该开关约束。

### M3 已支持（CI）

- **流水线**：按分支 / 状态筛选的列表，运行中流水线每 3 秒自动刷新
- **流水线详情**：阶段与作业树，按 stage 分组聚合状态
- **作业日志**：基于 `Range: bytes=<offset>-` 的增量拉取，自动跟随滚动，支持行号、错误/警告着色、仅看问题、关键字过滤、复制与下载
- **作业操作**：重试、取消、手动执行（`when: manual`）
- **流水线操作**：重试失败作业、取消、按 ref 触发新流水线（支持自定义变量）
- **MCP CI 工具**：`gitlab_list_pipelines`、`gitlab_get_pipeline_jobs`、`gitlab_get_job_log`、`gitlab_retry_job`、`gitlab_cancel_job`、`gitlab_play_job`、`gitlab_retry_pipeline`、`gitlab_cancel_pipeline`、`gitlab_create_pipeline`

> 日志单次最多拉取 2 MiB，未读完会保留 offset 供下一轮续拉。

### M4 已支持（Issues / Todos / 账号密码）

- **Issue 列表**：我的待办、分配给我、我创建的、当前项目四种视角，支持状态与关键字筛选
- **Issue 详情**：描述、标签、里程碑、指派、评论流，支持关闭 / 重开、编辑标题与描述、发表评论
- **新建 Issue**：标题、描述、标签
- **Todos**：待办列表与「标记完成」
- **MCP 工具**：`gitlab_list_issues`、`gitlab_get_issue`、`gitlab_list_todos`、`gitlab_create_issue`、`gitlab_comment_issue`、`gitlab_mark_todo_done`

#### 账号密码认证

选择「账号密码」后，插件调用 `POST /oauth/token`（Resource Owner Password Credentials）换取 access token，之后用 `Authorization: Bearer` 调用 API；token 到期前 60 秒自动用 `refresh_token` 刷新，刷新失败会回落到账号密码重新换取。

三个限制：

1. **开启两步验证（2FA）的账号无法使用**，GitLab 会直接拒绝，请改用访问令牌。
2. GitLab.com 自 2025-04 起要求客户端认证，需在连接里填「OAuth 应用 ID / 密钥」（自建实例通常留空即可）。
3. 密码仅保存在连接密钥与 sidecar 进程内存中，用于换取和刷新 token；不会落盘、不会写入日志。

### M5 已支持（制品 / 测试报告 / 成员 / 代码搜索）

- **测试报告**：流水线详情展示总数、通过、失败、跳过、总耗时，并列出失败用例（文件、耗时）
- **作业制品**：在作业日志面板下载 zip；单次上限 8 MiB，超限提示到 GitLab 获取
- **成员**：项目成员列表（含从上层组继承）、按名称搜索、角色徽章（Guest → Owner）
- **代码搜索**：在默认分支内搜索，结果显示文件路径、行号与匹配片段
- **MCP 工具**：`gitlab_list_members`、`gitlab_search_code`、`gitlab_get_test_report`

> 分支保护、制品目录树浏览暂未实现，需要的场景请走 GitLab 网页。

## 外部 MCP

manifest 声明 `external_tools: true`，并把 35 个工具按工具集与访问级别显式登记（DBX 外部 MCP 是严格契约：未登记的工具不会对外暴露）。外部 MCP 客户端经由 `dbx-mcp` 调用，与内置 AI 共用同一套工具实现。

| 工具集 | 工具数 | 级别 |
|---|---|---|
| Projects and code | 7 | read |
| Merge requests | 12 | read / write / full（`gitlab_merge_mr`） |
| Pipelines and jobs | 10 | read / write |
| Issues and todos | 6 | read / write |

启用步骤：

1. 安装插件并添加 GitLab 连接；
2. 打开「设置 → MCP → 能力」，找到 **GitLab Console** 卡片并打开「外部 MCP」开关；
3. 首次开启时选择授权上限：只读 / 读写 / 完全，默认取插件声明最高级别与「读写」中较低者（即 `write`）；
4. 需要 AI 合并 MR（`full`）时把上限提到「完全」，并按需开启调用确认。

注意：

- 授权上限之外的写工具保持隐藏；只读连接下写工具同样不可见。
- 插件升级后新增的非只读工具、或级别升高的工具需要重新确认一次，不会静默扩大权限。
- 单工具可在同一页面逐个关闭；工具总数 35（≤ 40），默认按 `flat` 直接列出。

## 安装

在 DBX 插件中心搜索 **GitLab Console**，安装后添加 GitLab 连接。

插件支持 Windows、macOS 与 Linux。打包 CLI 要求在目标 OS/架构的机器上打包（`target` 与构建主机不一致会直接报错），因此发布由 CI 在 5 个原生 runner 上分别打包，一次发版产出全部产物：

| 平台 | 产物 |
|---|---|
| Windows x64 | `io.github.nixing178.gitlab-<version>-windows-x64.dbxp` |
| macOS Apple Silicon | `io.github.nixing178.gitlab-<version>-darwin-arm64.dbxp` |
| macOS Intel | `io.github.nixing178.gitlab-<version>-darwin-x64.dbxp` |
| Linux x64 | `io.github.nixing178.gitlab-<version>-linux-x64.dbxp` |
| Linux ARM64 | `io.github.nixing178.gitlab-<version>-linux-arm64.dbxp` |

本地开发包可在「插件中心 → 设置 → 允许未签名开发包」后，从 `dist/` 下对应平台的 dbxp 安装。

## 连接配置

| 字段 | 说明 |
|---|---|
| 协议 / IP 域名 / 端口 | 实例地址，例如 `https` + `gitlab.example.com` + `443` |
| 上下文路径 | 实例部署在子路径时填写，例如 `/gitlab` |
| 认证方式 | `访问令牌` 或 `账号密码`（见下方说明） |
| 访问令牌 | Personal / Project / Group Access Token，scope 至少 `read_api`，需要写操作时用 `api` |
| 只读模式 | 默认开启；M2 起写操作需显式关闭 |

自签证书实例可开启「跳过 TLS 证书校验」，仅建议在内网使用。

## 开发

```powershell
pnpm install --ignore-scripts
pnpm typecheck
pnpm test          # 前端纯函数测试（vitest）
pnpm build         # 版本校验 + MCP 工具声明校验 + 类型检查 + 单文件产物
node scripts/check-mcp-tools.mjs   # 单独校验 manifest.tools 与 mcp.go 工具清单一致

cd backend
go test ./...      # GitLab 客户端与 RPC 层测试（httptest 模拟）
go build ./...

# 打包（PATH 需包含 go；产物 target 取当前主机，CLI 不允许跨平台打包）
npx --yes @dbx-app/plugin-cli@0.1.9 package .

# 显式指定 target（必须与构建主机一致，例如在 Apple Silicon 上）
$env:DBX_PLUGIN_TARGET='darwin-arm64'; npx --yes @dbx-app/plugin-cli@0.1.9 package .

# 结构校验（target 默认取包名后缀，可用 --target 覆盖）
python scripts/verify-package.py dist/io.github.nixing178.gitlab-0.3.1-windows-x64.dbxp

# 身份校验（省略路径时按当前主机推断 bin/<target>/ 下的可执行文件）
node scripts/verify-sidecar-identity.mjs dist/unpacked/bin/windows-x64/dbx-plugin-gitlab.exe
```

`main` 分支受保护：改动需走 PR，且 `web`（typecheck / vitest / build）与 `backend`（vet / race test / build / darwin·linux 交叉编译）两个必需检查通过才能合并；禁止强推与删除分支。

## 结构

```text
manifest.json                 # connection-provider + workbench + mcp
src/                          # Vue3 UI
  lib/                        # diff 解析、Markdown、状态映射、格式化（vitest 覆盖）
  components/                 # 项目、文件树、提交、MR 列表/详情/diff/讨论
backend/internal/gitlab/      # REST 客户端、分页、错误映射
backend/internal/rpc/         # RPC 分发、只读闸门、MCP 工具
```

## 设计要点

- **只读闸门在 RPC 层**：写操作统一经过 `requireWritable`，不依赖 UI 灰化。
- **项目标识**：统一使用完整路径（`group/project`），由后端做 URL 编码。
- **错误语义**：`unauthorized` / `forbidden` / `not_found` / `conflict` / `rate_limited` / `server_error`，UI 按 kind 本地化。
- **Markdown 安全**：先 HTML 转义，链接仅放行 `http/https/mailto`。
- **文件读取上限**：默认 1 MiB，超过只返回元信息，避免 UI 卡死。
