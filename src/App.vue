<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import logoUrl from "../assets/plugin.svg";
import { describeError, gitlabApi, setConnectionId } from "./api";
import AppSidebar from "./components/AppSidebar.vue";
import BranchList from "./components/BranchList.vue";
import BranchNewView from "./components/BranchNewView.vue";
import Breadcrumbs from "./components/Breadcrumbs.vue";
import CommitView from "./components/CommitView.vue";
import Icon from "./components/Icon.vue";
import IssueList from "./components/IssueList.vue";
import IssueView from "./components/IssueView.vue";
import MergeRequestList from "./components/MergeRequestList.vue";
import MergeRequestView from "./components/MergeRequestView.vue";
import MrNewView from "./components/MrNewView.vue";
import PipelineView from "./components/PipelineView.vue";
import ProjectList from "./components/ProjectList.vue";
import ProjectView from "./components/ProjectView.vue";
import { syncLocale, t } from "./i18n";
import type {
  BranchScope,
  IssueScope,
  MrScope,
  MrTab,
  ProjectView as ProjectViewName,
} from "./lib/nav";
import { projectGate } from "./lib/projectGate";
import {
  back,
  canGoBack,
  canGoForward,
  crumbs,
  currentProjectPath,
  forward,
  go,
  replace,
  route,
} from "./state/nav";
import { rememberProject } from "./state/workspace";
import type { GitLabProject, GitLabUser, Issue, MergeRequest, Todo } from "./types";

const connectionName = ref("");
const identity = ref<GitLabUser | null>(null);
const version = ref("");
const readOnly = ref(true);
const error = ref("");
const booted = ref(false);

const projectCache = ref<Record<string, GitLabProject>>({});
const projectMissing = ref("");

const activeProject = computed<GitLabProject | null>(() => {
  const path = currentProjectPath.value;
  return path ? projectCache.value[path] ?? null : null;
});

const gate = computed(() =>
  projectGate(
    currentProjectPath.value,
    projectMissing.value,
    Boolean(projectCache.value[currentProjectPath.value]),
  ),
);

const projectView = computed<ProjectViewName>(() =>
  route.value.name === "project" ? route.value.view : "overview",
);
const mrsScope = computed<MrScope>(() =>
  route.value.name === "mrs" ? route.value.scope : "reviewer",
);
const mrsState = computed(() => (route.value.name === "mrs" ? route.value.state : "opened"));
const mrsSearch = computed(() => (route.value.name === "mrs" ? route.value.search : ""));
const mrsOrderBy = computed(() => (route.value.name === "mrs" ? route.value.orderBy ?? "" : ""));
const mrsSort = computed(() => (route.value.name === "mrs" ? route.value.sort ?? "" : ""));
const issuesScope = computed<IssueScope>(() =>
  route.value.name === "issues" ? route.value.scope : "todos",
);
const issuesState = computed(() => (route.value.name === "issues" ? route.value.state : "opened"));
const issuesSearch = computed(() => (route.value.name === "issues" ? route.value.search : ""));
const branchScope = computed<BranchScope>(() =>
  route.value.name === "branches" ? route.value.scope : "overview",
);
const branchSearch = computed(() => (route.value.name === "branches" ? route.value.search : ""));
/** 项目代码页锁定的分支；空表示用默认分支。 */
const projectRef = computed(() => (route.value.name === "project" ? route.value.ref ?? "" : ""));
const pipelineId = computed(() => (route.value.name === "pipeline" ? route.value.pipelineId : null));
const jobId = computed(() =>
  route.value.name === "pipeline" ? route.value.jobId ?? null : null,
);

/** 连接名与当前账号同名时不再重复显示。 */
const showConnectionName = computed(() => {
  if (!connectionName.value) return false;
  const user = identity.value;
  if (!user) return true;
  return ![user.name, user.username].filter(Boolean).includes(connectionName.value);
});

function applyTheme(): void {
  const theme = window.dbxPlugin?.theme;
  if (!theme?.appearance) return;
  document.documentElement.dataset.theme = theme.appearance;
  for (const [key, value] of Object.entries(theme.tokens ?? {})) {
    if (/^--(color|radius|font)-/.test(key)) {
      document.documentElement.style.setProperty(key, value);
    }
  }
}

function handleEnv(): void {
  syncLocale(window.dbxPlugin?.locale);
  applyTheme();
}

function openProject(project: GitLabProject): void {
  projectCache.value[project.path_with_namespace] = project;
  rememberProject(project);
  go({ name: "project", project: project.path_with_namespace, view: "overview" });
}

function setProjectView(view: ProjectViewName): void {
  if (route.value.name !== "project") return;
  replace({ ...route.value, view });
}

/** 项目页加星后同步缓存，切走再回来不会退回旧计数。 */
function onProjectStarChanged(payload: { starred: boolean; starCount: number }): void {
  const path = currentProjectPath.value;
  const project = projectCache.value[path];
  if (!project) return;
  projectCache.value[path] = {
    ...project,
    starred: payload.starred,
    star_count: payload.starCount,
  };
}

/** 提交列表 / MR 提交 Tab / 上级提交都走这里，统一用当前项目上下文。 */
function openCommit(sha: string): void {
  const path = currentProjectPath.value;
  if (!path || !sha) return;
  error.value = "";
  go({ name: "commit", project: path, sha });
}

/** 全局 MR / Issue 可能属于别的项目，先按 project_id 补项目详情。 */
async function ensureProject(projectId: number): Promise<string> {
  const cached = Object.values(projectCache.value).find((item) => item.id === projectId);
  if (cached) return cached.path_with_namespace;
  try {
    const project = await gitlabApi.project(String(projectId));
    projectCache.value[project.path_with_namespace] = project;
    return project.path_with_namespace;
  } catch (err) {
    error.value = describeError(err);
    return "";
  }
}

async function openMergeRequest(mr: MergeRequest): Promise<void> {
  error.value = "";
  const path = await ensureProject(mr.project_id);
  if (!path) return;
  rememberProject({ path_with_namespace: path, name: path.split("/").pop() ?? path });
  go({ name: "mr", project: path, iid: mr.iid, tab: "overview" });
}

function openIssue(issue: Issue, projectPath: string): void {
  error.value = "";
  const fallback = route.value.name === "issues" ? route.value.project : "";
  const path = projectPath || fallback;
  if (!path) {
    error.value = "Cannot resolve the project of this issue.";
    return;
  }
  go({ name: "issue", project: path, iid: issue.iid });
}

function openTodo(todo: Todo): void {
  error.value = "";
  const path = todo.project?.path_with_namespace ?? "";
  const iid = todo.target?.iid ?? 0;
  if (!path || !iid) {
    error.value = "This todo is not linked to an issue.";
    return;
  }
  go({ name: "issue", project: path, iid });
}

function setMrFilter(
  patch: Partial<{ scope: MrScope; state: string; search: string; orderBy: string; sort: string }>,
): void {
  if (route.value.name !== "mrs") return;
  replace({ ...route.value, ...patch });
}

function setMrTab(tab: MrTab): void {
  if (route.value.name !== "mr") return;
  replace({ ...route.value, tab });
}

/** 新建 MR 从列表按钮进入，走路由而不是弹窗，返回时列表筛选不丢。 */
function openCreateMr(): void {
  const path = currentProjectPath.value;
  if (!path) return;
  error.value = "";
  go({ name: "mrNew", project: path });
}

function backToMrList(): void {
  const path = currentProjectPath.value;
  if (!path) return;
  go({ name: "mrs", project: path, scope: "project", state: "opened", search: "" });
}

/** 新建分支走整页路由，取消后回到分支列表。 */
function openCreateBranch(): void {
  const path = currentProjectPath.value;
  if (!path) return;
  error.value = "";
  go({ name: "branchNew", project: path });
}

function backToBranchList(): void {
  const path = currentProjectPath.value;
  if (!path) return;
  go({ name: "branches", project: path, scope: "overview", search: "" });
}

/** 创建成功后回列表并按新分支名筛选，保证一眼能看到。 */
function onBranchCreated(name: string): void {
  const path = currentProjectPath.value;
  if (!path) return;
  go({ name: "branches", project: path, scope: "all", search: name });
}

/** 点分支名：打开锁定该 ref 的项目代码页，返回时回到分支列表。 */
function openBranchRef(name: string): void {
  const path = currentProjectPath.value;
  if (!path || !name) return;
  error.value = "";
  go({ name: "project", project: path, view: "overview", ref: name });
}

/** 项目页切换分支只改当前页参数，前进/后退仍能复现。 */
function setProjectRef(value: string): void {
  if (route.value.name !== "project") return;
  replace({ ...route.value, ref: value });
}

function openPipelineFromMr(pipelineId: number): void {
  if (route.value.name !== "mr") return;
  go({ name: "pipeline", project: route.value.project, pipelineId });
}

function setIssueFilter(patch: Partial<{ scope: IssueScope; state: string; search: string }>): void {
  if (route.value.name !== "issues") return;
  replace({ ...route.value, ...patch });
}

/** 分支页的页签与筛选都进路由，返回列表时条件不丢。 */
function setBranchFilter(patch: Partial<{ scope: BranchScope; search: string }>): void {
  if (route.value.name !== "branches") return;
  replace({ ...route.value, ...patch });
}

function onPipelineNavigate(id: number | null): void {
  if (route.value.name !== "pipeline" && route.value.name !== "pipelines") return;
  const path = route.value.project;
  go(
    id
      ? { name: "pipeline", project: path, pipelineId: id }
      : { name: "pipelines", project: path, status: "", ref: "" },
  );
}

/** 作业抽屉的开关只改路由参数，不压栈。 */
function onJobSelect(id: number | null): void {
  if (route.value.name !== "pipeline") return;
  replace(
    id
      ? { ...route.value, jobId: id }
      : { name: "pipeline", project: route.value.project, pipelineId: route.value.pipelineId },
  );
}

/**
 * 详情页路由只带 path，项目详情按需拉一次并缓存。
 * 列表对象不带 starred（GitLab 列表接口没有该字段），进项目后补一次详情再合并。
 */
watch(
  [currentProjectPath, booted],
  async ([path, ready]) => {
    projectMissing.value = "";
    if (!ready || !path) return;
    const cached = projectCache.value[path];
    if (cached && cached.starred !== undefined) return;
    try {
      const detail = await gitlabApi.project(path);
      projectCache.value[path] = cached ? { ...cached, ...detail } : detail;
    } catch (err) {
      if (cached) return;
      projectMissing.value = path;
      error.value = describeError(err);
    }
  },
  { immediate: true },
);

onMounted(async () => {
  const plugin = window.dbxPlugin;
  if (!plugin) {
    error.value = "window.dbxPlugin is undefined";
    return;
  }
  await plugin.ready;
  setConnectionId(plugin.context?.connectionId);
  connectionName.value = String(plugin.context?.connectionName ?? "");
  syncLocale(plugin.locale);
  applyTheme();
  window.addEventListener("dbx-plugin-env", handleEnv);
  booted.value = true;

  try {
    const result = await gitlabApi.whoami();
    identity.value = result.user;
    readOnly.value = result.read_only;
    version.value = result.version;
    if (!connectionName.value) {
      connectionName.value = result.user?.name || result.user?.username || "";
    }
  } catch (err) {
    error.value = describeError(err);
  }
});
</script>

<template>
  <div class="shell">
    <header class="topbar">
      <button class="brand" type="button" :title="t('tabProjects')" @click="go({ name: 'projects' })">
        <img :src="logoUrl" alt="" />
        <span class="brand-text">{{ t("appTitle") }}</span>
      </button>

      <div class="nav-arrows">
        <button
          class="nav-arrow"
          type="button"
          :disabled="!canGoBack"
          :title="t('navBack')"
          :aria-label="t('navBack')"
          @click="back()"
        >
          <Icon name="chevron-left" :size="16" />
        </button>
        <button
          class="nav-arrow"
          type="button"
          :disabled="!canGoForward"
          :title="t('navForward')"
          :aria-label="t('navForward')"
          @click="forward()"
        >
          <Icon name="chevron" :size="16" />
        </button>
      </div>

      <Breadcrumbs :crumbs="crumbs" @navigate="go" />

      <span class="spacer"></span>

      <span v-if="identity" class="connection" :title="identity.username">
        {{ identity.name || identity.username }}
      </span>
      <span v-if="version" class="badge" data-tone="neutral">GitLab {{ version }}</span>
      <span class="badge" :data-tone="readOnly ? 'neutral' : 'warning'">
        {{ readOnly ? t("readOnly") : t("writeEnabled") }}
      </span>
      <span v-if="showConnectionName" class="connection connection-name" :title="connectionName">
        {{ connectionName }}
      </span>
    </header>

    <div class="shell-body">
      <AppSidebar />

      <main class="main">
        <div v-if="error" class="error-banner">
          <span class="grow">{{ error }}</span>
          <button class="btn ghost" type="button" @click="error = ''">×</button>
        </div>

        <div v-if="gate === 'loading'" class="loading">{{ t("loadingProject") }}</div>
        <div v-else-if="gate === 'missing'" class="card">
          <div class="empty">{{ t("projectMissing") }}</div>
        </div>

        <template v-else>
          <ProjectList v-if="route.name === 'projects'" :read-only="readOnly" @open="openProject" />

          <ProjectView
            v-else-if="route.name === 'project' && activeProject"
            :project="activeProject"
            :view="projectView"
            :read-only="readOnly"
            :active-ref="projectRef"
            :me="identity"
            @update:view="setProjectView"
            @update:ref="setProjectRef"
            @open-commit="(commit) => openCommit(commit.id)"
            @star-changed="onProjectStarChanged"
          />

          <BranchNewView
            v-else-if="route.name === 'branchNew' && activeProject"
            :project="activeProject"
            :read-only="readOnly"
            @created="onBranchCreated"
            @cancel="backToBranchList"
          />

          <BranchList
            v-else-if="route.name === 'branches' && activeProject"
            :project="activeProject"
            :scope="branchScope"
            :search="branchSearch"
            :read-only="readOnly"
            @update:scope="(value) => setBranchFilter({ scope: value })"
            @update:search="(value) => setBranchFilter({ search: value })"
            @open-commit="openCommit"
            @new-branch="openCreateBranch"
            @open-branch="openBranchRef"
          />

          <CommitView
            v-else-if="route.name === 'commit'"
            :project-path="route.project"
            :sha="route.sha"
            @back="go({ name: 'project', project: route.project, view: 'commits' })"
            @open-commit="openCommit"
            @open-mr="openMergeRequest"
          />

          <MergeRequestView
            v-else-if="route.name === 'mr' && activeProject"
            :project="activeProject"
            :iid="route.iid"
            :tab="route.tab"
            :read-only="readOnly"
            :me="identity"
            @update:tab="setMrTab"
            @open-pipeline="openPipelineFromMr"
            @open-commit="openCommit"
            @back="
              go({
                name: 'mrs',
                project: activeProject.path_with_namespace,
                scope: 'project',
                state: 'opened',
                search: '',
              })
            "
          />

          <MrNewView
            v-else-if="route.name === 'mrNew' && activeProject"
            :project="activeProject"
            :me="identity"
            :read-only="readOnly"
            @created="openMergeRequest"
            @cancel="backToMrList"
            @open-commit="openCommit"
          />

          <MergeRequestList
            v-else-if="route.name === 'mrs'"
            :project="activeProject"
            :read-only="readOnly"
            :scope="mrsScope"
            :state="mrsState"
            :search="mrsSearch"
            :order-by="mrsOrderBy"
            :sort="mrsSort"
            @create="openCreateMr"
            @open="openMergeRequest"
            @update:scope="(value) => setMrFilter({ scope: value })"
            @update:state="(value) => setMrFilter({ state: value })"
            @update:search="(value) => setMrFilter({ search: value })"
            @update:order-by="(value) => setMrFilter({ orderBy: value })"
            @update:sort="(value) => setMrFilter({ sort: value })"
          />

          <IssueView
            v-else-if="route.name === 'issue'"
            :project-path="route.project"
            :iid="route.iid"
            :read-only="readOnly"
            @back="
              go({
                name: 'issues',
                project: route.project,
                scope: 'project',
                state: 'opened',
                search: '',
              })
            "
          />

          <IssueList
            v-else-if="route.name === 'issues' || route.name === 'todos'"
            :project="activeProject"
            :read-only="readOnly"
            :scope="issuesScope"
            :state="issuesState"
            :search="issuesSearch"
            @open="openIssue"
            @open-todo="openTodo"
            @update:scope="(value) => setIssueFilter({ scope: value })"
            @update:state="(value) => setIssueFilter({ state: value })"
            @update:search="(value) => setIssueFilter({ search: value })"
          />

          <PipelineView
            v-else-if="(route.name === 'pipelines' || route.name === 'pipeline') && activeProject"
            :project="activeProject"
            :read-only="readOnly"
            :initial-pipeline-id="pipelineId"
            :initial-job-id="jobId"
            @navigate="onPipelineNavigate"
            @select-job="onJobSelect"
            @open-commit="openCommit"
          />
        </template>
      </main>
    </div>
  </div>
</template>
