<script setup lang="ts">
import { computed } from "vue";
import { locale, t } from "../i18n";
import { formatBytes } from "../lib/format";
import type { ProjectView } from "../lib/nav";
import type { GitLabProject, RepoOverview } from "../types";
import Icon from "./Icon.vue";

const props = defineProps<{
  project: GitLabProject;
  overview: RepoOverview | null;
}>();

const emit = defineEmits<{ "open-view": [view: ProjectView] }>();

const numberFormat = computed(
  () => new Intl.NumberFormat(locale.value === "zh" ? "zh-CN" : "en-US"),
);

function count(value: number | null | undefined): string {
  if (value === null || value === undefined || value < 0) return "-";
  return numberFormat.value.format(value);
}

const storage = computed(() => {
  const size = props.overview?.storage_size || props.overview?.repository_size || 0;
  return size > 0 ? formatBytes(size) : "-";
});

const createdAt = computed(() => {
  const iso = props.overview?.created_at || props.project.created_at || "";
  const time = iso ? Date.parse(iso) : Number.NaN;
  if (Number.isNaN(time)) return "-";
  return new Date(time).toLocaleDateString(locale.value === "zh" ? "zh-CN" : "en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
});

const shortcuts: Array<{ view: ProjectView; label: () => string; icon: string }> = [
  { view: "files", label: () => t("browseFiles"), icon: "folder" },
  { view: "commits", label: () => t("commits"), icon: "commit" },
  { view: "members", label: () => t("members"), icon: "users" },
  { view: "search", label: () => t("codeSearch"), icon: "search" },
];
</script>

<template>
  <div class="gl-side-card">
    <h2 class="gl-side-title">{{ t("projectInfo") }}</h2>

    <ul class="gl-side-stats">
      <li>
        <Icon name="commit" :size="15" />
        <span>{{ t("commitCount", { count: count(overview?.commit_count) }) }}</span>
      </li>
      <li>
        <Icon name="branch" :size="15" />
        <span>{{ t("branchCount", { count: count(overview?.branch_count) }) }}</span>
      </li>
      <li>
        <Icon name="tag" :size="15" />
        <span>{{ t("tagCount", { count: count(overview?.tag_count) }) }}</span>
      </li>
      <li>
        <Icon name="disk" :size="15" />
        <span>{{ storage }}</span>
      </li>
    </ul>

    <div class="gl-side-divider"></div>

    <ul class="gl-side-shortcuts">
      <li v-for="entry in shortcuts" :key="entry.view">
        <button type="button" @click="emit('open-view', entry.view)">
          <span class="gl-side-plus" aria-hidden="true">+</span>
          <Icon :name="entry.icon" :size="15" />
          <span>{{ entry.label() }}</span>
        </button>
      </li>
    </ul>

    <div class="gl-side-divider"></div>
    <div class="gl-side-meta">{{ t("createdOn", { date: createdAt }) }}</div>
    <div v-if="project.description" class="gl-side-desc">{{ project.description }}</div>
  </div>
</template>
