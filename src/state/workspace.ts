import { ref, watch } from "vue";
import { readJson, writeJson } from "../lib/storage";
import { normalizePins, togglePin } from "../lib/navPins";
import type { ProjectNav } from "../lib/nav";
import type { GitLabProject } from "../types";

export interface RecentProject {
  path: string;
  name: string;
}

interface Persisted {
  recent: RecentProject[];
  collapsed: boolean;
  /** 已钉选置顶的项目内导航；用户级偏好，所有项目共用。 */
  pinned: ProjectNav[];
  /** 「已钉选」分组是否展开。 */
  pinnedOpen: boolean;
}

const STORAGE_KEY = "dbx.gitlab.workspace.v1";
const MAX_RECENT = 8;

// 旧版本没有 pinned / pinnedOpen 字段，按缺省值兜底。
const persisted = readJson<Partial<Persisted>>(STORAGE_KEY, {});

export const recentProjects = ref<RecentProject[]>(
  Array.isArray(persisted.recent)
    ? persisted.recent.filter((item) => item && typeof item.path === "string")
    : [],
);
export const sidebarCollapsed = ref(Boolean(persisted.collapsed));
export const pinnedProjectNav = ref<ProjectNav[]>(normalizePins(persisted.pinned));
export const pinnedGroupOpen = ref(persisted.pinnedOpen !== false);

/** 项目列表的筛选条件提到模块级：侧栏入口与列表页共享，返回列表不丢条件。 */
export type ProjectScope = "member" | "starred" | "owned" | "inactive";

export const projectListScope = ref<ProjectScope>("member");
export const projectListSearch = ref("");

/** 最近访问置顶，去重，最多 8 条。 */
export function rememberProject(project: Pick<GitLabProject, "path_with_namespace" | "name">): void {
  const entry: RecentProject = {
    path: project.path_with_namespace,
    name: project.name || project.path_with_namespace,
  };
  recentProjects.value = [entry, ...recentProjects.value.filter((item) => item.path !== entry.path)].slice(
    0,
    MAX_RECENT,
  );
}

export function toggleSidebar(): void {
  sidebarCollapsed.value = !sidebarCollapsed.value;
}

/** 钉选/取消钉选某个项目内入口：钉了的排到左栏顶部。 */
export function togglePinnedProjectNav(id: ProjectNav): void {
  pinnedProjectNav.value = togglePin(pinnedProjectNav.value, id);
}

/** 折叠 / 展开「已钉选」分组。 */
export function togglePinnedGroup(): void {
  pinnedGroupOpen.value = !pinnedGroupOpen.value;
}

watch(
  [recentProjects, sidebarCollapsed, pinnedProjectNav, pinnedGroupOpen],
  () => {
    writeJson(STORAGE_KEY, {
      recent: recentProjects.value,
      collapsed: sidebarCollapsed.value,
      pinned: pinnedProjectNav.value,
      pinnedOpen: pinnedGroupOpen.value,
    } satisfies Persisted);
  },
  { deep: true },
);
