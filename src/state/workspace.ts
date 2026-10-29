import { ref, watch } from "vue";
import { readJson, writeJson } from "../lib/storage";
import type { GitLabProject } from "../types";

export interface RecentProject {
  path: string;
  name: string;
}

interface Persisted {
  recent: RecentProject[];
  collapsed: boolean;
}

const STORAGE_KEY = "dbx.gitlab.workspace.v1";
const MAX_RECENT = 8;

const persisted = readJson<Persisted>(STORAGE_KEY, { recent: [], collapsed: false });

export const recentProjects = ref<RecentProject[]>(
  Array.isArray(persisted.recent)
    ? persisted.recent.filter((item) => item && typeof item.path === "string")
    : [],
);
export const sidebarCollapsed = ref(Boolean(persisted.collapsed));

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

watch(
  [recentProjects, sidebarCollapsed],
  () => {
    writeJson(STORAGE_KEY, {
      recent: recentProjects.value,
      collapsed: sidebarCollapsed.value,
    } satisfies Persisted);
  },
  { deep: true },
);
