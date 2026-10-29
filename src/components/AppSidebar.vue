<script setup lang="ts">
import { computed, ref } from "vue";
import { t } from "../i18n";
import { activeProjectNav, activeSection, currentProjectPath, go, goSection } from "../state/nav";
import {
  projectListScope,
  projectListSearch,
  recentProjects,
  rememberProject,
  sidebarCollapsed,
  toggleSidebar,
  type ProjectScope,
} from "../state/workspace";
import type { ProjectNav, Section } from "../lib/nav";
import Icon from "./Icon.vue";

const project = currentProjectPath;
const collapsed = sidebarCollapsed;
const search = ref("");

const initials = computed(() => {
  const name = project.value.split("/").pop() ?? "";
  return name.slice(0, 2).toUpperCase() || "GL";
});

const filteredRecent = computed(() => {
  const keyword = search.value.trim().toLowerCase();
  if (!keyword) return recentProjects.value;
  return recentProjects.value.filter(
    (item) =>
      item.name.toLowerCase().includes(keyword) || item.path.toLowerCase().includes(keyword),
  );
});

interface NavEntry {
  id: Section;
  label: () => string;
  icon: string;
}

const workspaceEntries: NavEntry[] = [
  { id: "issues", label: () => t("issues"), icon: "issue" },
  { id: "mrs", label: () => t("mergeRequests"), icon: "merge" },
  { id: "pipelines", label: () => t("pipelines"), icon: "pipeline" },
  { id: "todos", label: () => t("todos"), icon: "clock" },
];

interface ProjectEntry {
  id: ProjectNav;
  label: () => string;
  icon: string;
  open: () => void;
}

function openProjectView(view: "overview" | "files"): void {
  const path = project.value;
  if (!path) return;
  go({ name: "project", project: path, view });
}

const projectEntries = computed<ProjectEntry[]>(() => [
  {
    id: "code",
    label: () => t("projectNavCode"),
    icon: "code",
    open: () => openProjectView("overview"),
  },
  { id: "issues", label: () => t("issues"), icon: "issue", open: () => openProjectIssues() },
  { id: "mrs", label: () => t("mergeRequests"), icon: "merge", open: () => openProjectMrs() },
  { id: "ci", label: () => t("projectNavCi"), icon: "pipeline", open: () => openProjectCi() },
]);

function openProjectIssues(): void {
  const path = project.value;
  if (!path) return;
  go({ name: "issues", project: path, scope: "project", state: "opened", search: "" });
}

function openProjectMrs(): void {
  const path = project.value;
  if (!path) return;
  go({ name: "mrs", project: path, scope: "project", state: "opened", search: "" });
}

function openProjectCi(): void {
  const path = project.value;
  if (!path) return;
  go({ name: "pipelines", project: path, status: "", ref: "" });
}

function openProjects(scope: ProjectScope = "member"): void {
  projectListScope.value = scope;
  go({ name: "projects" });
}

function submitSearch(): void {
  projectListSearch.value = search.value.trim();
  go({ name: "projects" });
}

function openRecent(path: string, name: string): void {
  rememberProject({ path_with_namespace: path, name });
  go({ name: "project", project: path, view: "overview" });
}

function isProjectEntryActive(id: ProjectNav): boolean {
  return activeProjectNav.value === id;
}

function isProjectsActive(): boolean {
  return activeSection.value === "projects" && projectListScope.value !== "starred";
}

function isStarredActive(): boolean {
  return activeSection.value === "projects" && projectListScope.value === "starred";
}
</script>

<template>
  <aside class="sidebar gl-sidebar" :class="{ collapsed }">
    <div class="sidebar-scroll">
      <div class="gl-sidebar-head">
        <span class="gl-sidebar-brand sidebar-label">
          <span class="gl-sidebar-logo">GL</span>
          <span class="gl-sidebar-title">GitLab</span>
        </span>
        <button
          class="gl-sidebar-toggle"
          type="button"
          :title="collapsed ? t('expandSidebar') : t('collapseSidebar')"
          :aria-label="collapsed ? t('expandSidebar') : t('collapseSidebar')"
          @click="toggleSidebar"
        >
          <Icon name="menu" :size="16" />
        </button>
      </div>

      <label v-if="!collapsed" class="gl-sidebar-search">
        <Icon name="search" :size="14" />
        <input
          v-model="search"
          type="search"
          :placeholder="t('searchOrJump')"
          @keydown.enter.prevent="submitSearch"
        />
      </label>

      <template v-if="project">
        <button
          class="sidebar-back"
          type="button"
          :title="t('navAllProjects')"
          @click="openProjects('member')"
        >
          <Icon name="chevron-left" :size="14" />
          <span class="sidebar-label">{{ t("navAllProjects") }}</span>
        </button>

        <div class="sidebar-project">
          <span class="sidebar-avatar">{{ initials }}</span>
          <span class="sidebar-project-name sidebar-label" :title="project">{{ project }}</span>
        </div>

        <nav class="sidebar-nav" role="navigation">
          <button
            v-for="entry in projectEntries"
            :key="entry.id"
            class="sidebar-item"
            type="button"
            :class="{ active: isProjectEntryActive(entry.id) }"
            :title="collapsed ? entry.label() : undefined"
            :aria-current="isProjectEntryActive(entry.id) ? 'page' : undefined"
            @click="entry.open()"
          >
            <Icon :name="entry.icon" />
            <span class="sidebar-label">{{ entry.label() }}</span>
          </button>
        </nav>

        <div v-if="!collapsed" class="sidebar-group">{{ t("navWorkspace") }}</div>
        <div v-if="!collapsed" class="sidebar-nav">
          <button
            class="sidebar-item"
            type="button"
            :class="{ active: activeSection === 'todos' }"
            @click="goSection('todos')"
          >
            <Icon name="clock" />
            <span class="sidebar-label">{{ t("todos") }}</span>
          </button>
        </div>
      </template>

      <template v-else>
        <div class="sidebar-group sidebar-label">{{ t("navProjects") }}</div>
        <nav class="sidebar-nav" role="navigation">
          <button
            class="sidebar-item"
            type="button"
            :class="{ active: isProjectsActive() }"
            :title="collapsed ? t('navAllProjects') : undefined"
            @click="openProjects('member')"
          >
            <Icon name="folder" />
            <span class="sidebar-label">{{ t("navAllProjects") }}</span>
          </button>
          <button
            class="sidebar-item"
            type="button"
            :class="{ active: isStarredActive() }"
            :title="collapsed ? t('scopeStarred') : undefined"
            @click="openProjects('starred')"
          >
            <Icon name="star" />
            <span class="sidebar-label">{{ t("scopeStarred") }}</span>
          </button>
        </nav>

        <div class="sidebar-group sidebar-label">{{ t("navYourWork") }}</div>
        <nav class="sidebar-nav" role="navigation">
          <button
            v-for="entry in workspaceEntries"
            :key="entry.id"
            class="sidebar-item"
            type="button"
            :class="{ active: activeSection === entry.id }"
            :title="collapsed ? entry.label() : undefined"
            @click="goSection(entry.id)"
          >
            <Icon :name="entry.icon" />
            <span class="sidebar-label">{{ entry.label() }}</span>
          </button>
        </nav>

        <template v-if="filteredRecent.length">
          <div class="sidebar-group sidebar-label">{{ t("navRecent") }}</div>
          <button
            v-for="item in filteredRecent"
            :key="item.path"
            class="sidebar-item recent"
            type="button"
            :title="item.path"
            @click="openRecent(item.path, item.name)"
          >
            <Icon name="folder" />
            <span class="sidebar-label">{{ item.name }}</span>
          </button>
        </template>
        <div v-else-if="search.trim()" class="sidebar-empty sidebar-label">
          {{ t("noSearchResults") }}
        </div>
      </template>
    </div>
  </aside>
</template>
