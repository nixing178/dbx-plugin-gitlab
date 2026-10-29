<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t, type MessageKey } from "../i18n";
import { relativeTime } from "../lib/format";
import { hasStarred, setStarred } from "../lib/starredPaths";
import { projectListScope, projectListSearch, type ProjectScope } from "../state/workspace";
import Icon from "./Icon.vue";
import type { GitLabProject } from "../types";

const props = withDefaults(defineProps<{ readOnly?: boolean }>(), { readOnly: false });
const emit = defineEmits<{ open: [project: GitLabProject] }>();

type Scope = ProjectScope;

const items = ref<GitLabProject[]>([]);
const search = projectListSearch;
const scope = projectListScope;
const orderBy = ref("last_activity_at");
const sort = ref<"asc" | "desc">("desc");
const page = ref(1);
const hasMore = ref(false);
const total = ref(0);
const loading = ref(false);
const error = ref("");
const starredPaths = ref<string[]>([]);
const starredLoaded = ref(false);
const starringPath = ref("");
let debounce: number | undefined;

const scopes: Array<{ id: Scope; label: MessageKey }> = [
  { id: "member", label: "scopeMember" },
  { id: "starred", label: "scopeStarred" },
  { id: "owned", label: "scopePersonal" },
  { id: "inactive", label: "scopeInactive" },
];

const sorts: Array<{ value: string; label: MessageKey }> = [
  { value: "last_activity_at", label: "sortLastActivity" },
  { value: "name", label: "sortName" },
  { value: "created_at", label: "sortCreated" },
  { value: "star_count", label: "sortStars" },
];

/** 「未激活」= 最近 6 个月没有活动，与 GitLab 看板口径一致。 */
function inactiveBefore(): string {
  const date = new Date();
  date.setMonth(date.getMonth() - 6);
  return date.toISOString();
}

async function load(reset = true): Promise<void> {
  loading.value = true;
  error.value = "";
  if (reset) page.value = 1;
  try {
    const result = await gitlabApi.projects({
      search: search.value.trim(),
      page: page.value,
      perPage: 30,
      starred: scope.value === "starred",
      owned: scope.value === "owned",
      orderBy: orderBy.value,
      sort: sort.value,
      lastActivityBefore: scope.value === "inactive" ? inactiveBefore() : undefined,
    });
    items.value = reset ? result.items : [...items.value, ...result.items];
    hasMore.value = result.has_more;
    total.value = result.total || items.value.length;
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

/**
 * 列表接口不返回 starred，单独拉一次「已加星」集合（最多 3 页），
 * 供每行星标按钮判断实心/描边。失败时按未加星展示，点击后由接口结果校正。
 */
async function loadStarred(): Promise<void> {
  if (starredLoaded.value) return;
  try {
    const paths: string[] = [];
    for (let index = 1; index <= 3; index += 1) {
      const result = await gitlabApi.projects({ starred: true, page: index, perPage: 100 });
      paths.push(...result.items.map((item) => item.path_with_namespace));
      if (!result.has_more) break;
    }
    starredPaths.value = paths;
    starredLoaded.value = true;
  } catch {
    /* 拉不到就先按未加星展示 */
  }
}

function isStarred(project: GitLabProject): boolean {
  return hasStarred(starredPaths.value, project.path_with_namespace);
}

/** 行内星标：成功后更新集合与计数；在「已加星标」页签里取消则直接从列表移除。 */
async function toggleStar(project: GitLabProject): Promise<void> {
  if (props.readOnly || starringPath.value) return;
  const path = project.path_with_namespace;
  const next = !isStarred(project);
  starringPath.value = path;
  error.value = "";
  try {
    const result = await gitlabApi.starProject(path, next);
    starredPaths.value = setStarred(starredPaths.value, path, result.starred);
    if (result.star_count >= 0) project.star_count = result.star_count;
    if (!result.starred && scope.value === "starred") {
      items.value = items.value.filter((item) => item.path_with_namespace !== path);
      total.value = Math.max(0, total.value - 1);
    }
  } catch (err) {
    error.value = describeError(err);
  } finally {
    starringPath.value = "";
  }
}

function selectScope(next: Scope): void {
  if (scope.value === next) return;
  scope.value = next;
}

watch(scope, () => void load(true));
watch(search, onSearch);

function onSearch(): void {
  window.clearTimeout(debounce);
  debounce = window.setTimeout(() => void load(true), 300);
}

function toggleSort(): void {
  sort.value = sort.value === "asc" ? "desc" : "asc";
  void load(true);
}

function loadMore(): void {
  page.value += 1;
  void load(false);
}

function initial(project: GitLabProject): string {
  return (project.name || project.path_with_namespace).slice(0, 1).toUpperCase();
}

/** 用项目名生成稳定的浅色底 + 深色字，模拟 GitLab 的字母头像。 */
function avatarStyle(project: GitLabProject): Record<string, string> {
  let hue = 0;
  for (const char of project.name || project.path_with_namespace) {
    hue = (hue * 31 + char.charCodeAt(0)) % 360;
  }
  return {
    background: `hsl(${hue} 70% 92%)`,
    color: `hsl(${hue} 45% 32%)`,
  };
}

onMounted(() => {
  void load(true);
  void loadStarred();
});
onBeforeUnmount(() => window.clearTimeout(debounce));
</script>

<template>
  <section class="page">
    <div class="page-head">
      <h1>{{ t("tabProjects") }}</h1>
      <span class="muted small">{{ t("projectCount", { count: total }) }}</span>
    </div>

    <div class="project-tabs">
      <button
        v-for="option in scopes"
        :key="option.id"
        type="button"
        :class="{ active: scope === option.id }"
        @click="selectScope(option.id)"
      >
        {{ t(option.label) }}
      </button>
    </div>

    <div class="toolbar">
      <label class="search-field grow">
        <Icon name="search" :size="14" />
        <input v-model="search" type="search" :placeholder="t('projectSearchPlaceholder')" />
      </label>
      <select v-model="orderBy" class="select" @change="load(true)">
        <option v-for="option in sorts" :key="option.value" :value="option.value">
          {{ t(option.label) }}
        </option>
      </select>
      <button class="btn ghost sort-toggle" type="button" @click="toggleSort">
        {{ sort === "asc" ? "↑" : "↓" }}
      </button>
      <button class="btn" type="button" :disabled="loading" @click="load(true)">
        {{ t("refresh") }}
      </button>
    </div>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="load(true)">{{ t("retry") }}</button>
    </div>

    <div class="card">
      <div v-if="!items.length && loading" class="loading">{{ t("loading") }}</div>
      <div v-else-if="!items.length" class="empty">{{ t("noData") }}</div>
      <div v-else class="list">
        <div
          v-for="project in items"
          :key="project.id"
          class="list-item project-row"
          role="button"
          tabindex="0"
          @click="emit('open', project)"
          @keydown.enter="emit('open', project)"
          @keydown.space.prevent="emit('open', project)"
        >
          <span class="project-avatar" :style="avatarStyle(project)">{{ initial(project) }}</span>

          <span class="project-main">
            <span class="project-title">
              <span class="project-namespace">{{ project.namespace?.full_path }} /</span>
              <strong>{{ project.name }}</strong>
              <Icon v-if="project.visibility === 'private'" name="lock" :size="12" />
              <span v-if="project.archived" class="badge" data-tone="warning">
                {{ t("archived") }}
              </span>
            </span>
            <span v-if="project.description" class="project-desc">{{ project.description }}</span>
          </span>

          <span class="project-stats">
            <button
              class="project-star"
              :class="{ starred: isStarred(project) }"
              type="button"
              :disabled="readOnly || starringPath === project.path_with_namespace"
              :aria-pressed="isStarred(project)"
              :title="
                readOnly
                  ? t('readOnlyActionHint')
                  : isStarred(project)
                    ? t('unstarProject')
                    : t('starProject')
              "
              @click.stop="toggleStar(project)"
            >
              <Icon name="star" :size="13" />{{ project.star_count }}
            </button>
            <span :title="t('forks')">
              <Icon name="fork" :size="13" />{{ project.forks_count }}
            </span>
            <span :title="t('openIssues')">
              <Icon name="issue" :size="13" />{{ project.open_issues_count }}
            </span>
          </span>

          <span class="project-updated muted small">
            {{ t("updatedAt", { time: relativeTime(project.last_activity_at, locale) }) }}
          </span>
        </div>
      </div>
    </div>

    <div v-if="hasMore" class="footer-note">
      <button class="btn" type="button" :disabled="loading" @click="loadMore">
        {{ loading ? t("loading") : t("loadMore") }}
      </button>
    </div>
  </section>
</template>
