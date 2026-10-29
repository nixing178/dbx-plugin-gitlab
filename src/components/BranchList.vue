<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t, type MessageKey } from "../i18n";
import {
  branchTreeUrl,
  filterBranches,
  groupBranches,
  loadAllBranches,
  sortBranches,
} from "../lib/branches";
import { copyText } from "../lib/clipboard";
import { relativeTime } from "../lib/format";
import type { BranchScope } from "../lib/nav";
import Icon from "./Icon.vue";
import type { Branch, GitLabProject } from "../types";

/** 概览页每个分组最多展开多少条（与 GitLab 网页一致），其余切到对应页签看。 */
const OVERVIEW_LIMIT = 5;
const COPIED_MS = 1500;
/** 输入即筛的防抖：只改路由，不重新拉分支。 */
const SEARCH_DEBOUNCE_MS = 250;

const props = withDefaults(
  defineProps<{
    project: GitLabProject;
    scope: BranchScope;
    search: string;
    readOnly?: boolean;
  }>(),
  { readOnly: false },
);

const emit = defineEmits<{
  "update:scope": [value: BranchScope];
  "update:search": [value: string];
  "open-commit": [sha: string];
  "new-branch": [];
  "open-branch": [name: string];
}>();

const branches = ref<Branch[]>([]);
/** 下一批续拉页码；概览需要全量分支才能算出完整的活跃/非活跃分组。 */
const nextPage = ref(1);
const hasMore = ref(false);
const loading = ref(false);
const error = ref("");
const keyword = ref(props.search);
const copied = ref("");

/** 上一次请求的序号：搜索词连续变化时丢弃迟到的旧响应。 */
let requestSeq = 0;
let copiedTimer: number | undefined;
let searchTimer: number | undefined;

const scopes: Array<{ value: BranchScope; label: MessageKey }> = [
  { value: "overview", label: "branchTabOverview" },
  { value: "active", label: "branchTabActive" },
  { value: "stale", label: "branchTabStale" },
  { value: "all", label: "branchTabAll" },
];

/** 分支已全量加载，筛选在本地做，输入时不再打服务端。 */
const visibleBranches = computed(() => filterBranches(branches.value, props.search));
const groups = computed(() => groupBranches(visibleBranches.value));
const isEmpty = computed(() => !groups.value.active.length && !groups.value.stale.length);

interface BranchSection {
  key: string;
  label: MessageKey;
  scope: BranchScope;
  items: Branch[];
  more: boolean;
}

const sections = computed<BranchSection[]>(() => {
  const { active, stale } = groups.value;
  if (props.scope === "active") {
    return [{ key: "active", label: "branchActiveGroup", scope: "active", items: active, more: false }];
  }
  if (props.scope === "stale") {
    return [{ key: "stale", label: "branchStaleGroup", scope: "stale", items: stale, more: false }];
  }
  if (props.scope === "all") {
    return [
      {
        key: "all",
        label: "branchAllGroup",
        scope: "all",
        items: sortBranches(visibleBranches.value),
        more: false,
      },
    ];
  }
  return [
    {
      key: "active",
      label: "branchActiveGroup",
      scope: "active",
      items: active.slice(0, OVERVIEW_LIMIT),
      more: active.length > OVERVIEW_LIMIT,
    },
    {
      key: "stale",
      label: "branchStaleGroup",
      scope: "stale",
      items: stale.slice(0, OVERVIEW_LIMIT),
      more: stale.length > OVERVIEW_LIMIT,
    },
  ];
});

const webUrl = computed(() => (props.project.web_url || "").replace(/\/+$/, ""));
const rulesHref = computed(() => `${webUrl.value}/-/settings/repository`);

async function load(reset = true): Promise<void> {
  if (loading.value && !reset) return;
  const seq = ++requestSeq;
  loading.value = true;
  error.value = "";
  try {
    const result = await loadAllBranches(
      (page) => gitlabApi.branches(props.project.path_with_namespace, "", page, 100),
      { startPage: reset ? 1 : nextPage.value },
    );
    if (seq !== requestSeq) return;
    branches.value = reset ? result.items : [...branches.value, ...result.items];
    nextPage.value = result.nextPage;
    hasMore.value = result.truncated;
  } catch (err) {
    if (seq !== requestSeq) return;
    error.value = describeError(err);
  } finally {
    if (seq === requestSeq) loading.value = false;
  }
}

/** 筛选只写路由，列表用本地全量分支过滤，不再触发翻页。 */
function applySearch(): void {
  const next = keyword.value.trim();
  if (next !== props.search) emit("update:search", next);
}

function submitSearch(): void {
  window.clearTimeout(searchTimer);
  applySearch();
}

function openCommit(branch: Branch): void {
  const sha = branch.commit?.id;
  if (sha) emit("open-commit", sha);
}

async function copyName(name: string): Promise<void> {
  if (!(await copyText(name))) return;
  copied.value = name;
  window.clearTimeout(copiedTimer);
  copiedTimer = window.setTimeout(() => {
    if (copied.value === name) copied.value = "";
  }, COPIED_MS);
}

function shortSha(branch: Branch): string {
  return branch.commit?.short_id || branch.commit?.id?.slice(0, 8) || "";
}

watch(
  () => props.project.path_with_namespace,
  () => void load(true),
);

/** 外部改路由（返回、面包屑）时同步输入框，但不重复触发筛选。 */
watch(
  () => props.search,
  (value) => {
    if (value !== keyword.value.trim()) keyword.value = value;
  },
);

watch(keyword, () => {
  window.clearTimeout(searchTimer);
  searchTimer = window.setTimeout(applySearch, SEARCH_DEBOUNCE_MS);
});

onMounted(() => void load(true));

onBeforeUnmount(() => {
  window.clearTimeout(copiedTimer);
  window.clearTimeout(searchTimer);
});
</script>

<template>
  <section class="page gl-branches">
    <div class="gl-branches-head">
      <div class="tabs">
        <button
          v-for="option in scopes"
          :key="option.value"
          type="button"
          :class="{ active: scope === option.value }"
          @click="emit('update:scope', option.value)"
        >
          {{ t(option.label) }}
        </button>
      </div>

      <span class="grow"></span>

      <form class="gl-branches-filter" @submit.prevent="submitSearch">
        <input
          v-model="keyword"
          class="input"
          type="search"
          :placeholder="t('branchFilter')"
          :aria-label="t('branchFilter')"
        />
        <button class="gl-icon-btn" type="submit" :title="t('branchSearch')" :aria-label="t('branchSearch')">
          <Icon name="search" :size="14" />
        </button>
      </form>

      <a
        v-if="!readOnly"
        class="gl-btn subtle"
        :href="rulesHref"
        target="_blank"
        rel="noreferrer"
        :title="t('branchRules')"
      >
        {{ t("branchRules") }}
      </a>
      <button
        v-if="!readOnly"
        class="btn primary"
        type="button"
        :title="t('branchNewHint')"
        @click="emit('new-branch')"
      >
        {{ t("branchNew") }}
      </button>
      <button
        v-else
        class="btn primary"
        type="button"
        disabled
        :title="t('readOnlyActionHint')"
      >
        {{ t("branchNew") }}
      </button>
    </div>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="load(true)">{{ t("retry") }}</button>
    </div>

    <div class="card">
      <div v-if="loading && !branches.length" class="loading">{{ t("loading") }}</div>
      <div v-else-if="isEmpty" class="empty">
        {{ search ? t("noSearchResults") : t("noBranches") }}
      </div>
      <template v-else>
        <div v-for="section in sections" :key="section.key" class="gl-branch-section">
          <div v-if="scope !== 'all'" class="gl-branch-section-head">
            <h2 class="gl-branch-section-title">{{ t(section.label) }}</h2>
          </div>

          <div v-if="!section.items.length" class="gl-branch-none">{{ t("branchEmpty") }}</div>
          <div v-else class="list">
            <div
              v-for="branch in section.items"
              :key="branch.name"
              class="list-item gl-branch-row clickable"
              @click="emit('open-branch', branch.name)"
            >
              <Icon class="gl-branch-icon" name="branch" :size="15" />

              <div class="grow">
                <div class="title gl-branch-line">
                  <button
                    class="mono gl-branch-name gl-branch-name-btn"
                    type="button"
                    :title="t('branchOpenDetail')"
                    @click.stop="emit('open-branch', branch.name)"
                  >
                    {{ branch.name }}
                  </button>
                  <span v-if="branch.default" class="badge" data-tone="info">
                    {{ t("branchDefault") }}
                  </span>
                  <span v-if="branch.protected" class="badge" data-tone="neutral">
                    {{ t("branchProtected") }}
                  </span>
                </div>
                <div class="sub gl-branch-meta">
                  <button
                    v-if="branch.commit"
                    class="link mono small"
                    type="button"
                    :title="t('commit')"
                    @click.stop="openCommit(branch)"
                  >
                    {{ shortSha(branch) }}
                  </button>
                  <span class="gl-branch-commit" :title="branch.commit?.title || ''">
                    {{ branch.commit?.title || t("noCommits") }}
                  </span>
                  <span v-if="branch.commit" class="muted">
                    · {{ relativeTime(branch.commit.committed_date, locale) }}
                  </span>
                </div>
              </div>

              <div class="right gl-branch-actions" @click.stop>
                <button
                  class="gl-icon-btn"
                  type="button"
                  :title="copied === branch.name ? t('copied') : t('branchCopyName')"
                  :aria-label="t('branchCopyName')"
                  @click="copyName(branch.name)"
                >
                  {{ copied === branch.name ? t("copied") : t("copy") }}
                </button>
                <a
                  class="gl-icon-btn"
                  :href="branchTreeUrl(webUrl, branch.name)"
                  target="_blank"
                  rel="noreferrer"
                  :title="t('branchOpen')"
                  :aria-label="t('branchOpen')"
                >
                  <Icon name="external" :size="14" />
                </a>
                <button
                  v-if="branch.commit"
                  class="gl-icon-btn"
                  type="button"
                  :title="t('commit')"
                  :aria-label="t('commit')"
                  @click="openCommit(branch)"
                >
                  <Icon name="chevron" :size="14" />
                </button>
              </div>
            </div>
          </div>

          <div v-if="section.more" class="gl-branch-more">
            <button class="link-btn" type="button" @click="emit('update:scope', section.scope)">
              {{ section.key === "active" ? t("branchShowAllActive") : t("branchShowAllStale") }}
            </button>
          </div>
        </div>
      </template>

      <div v-if="hasMore" class="footer-note">
        <button class="btn" type="button" :disabled="loading" @click="load(false)">
          {{ t("loadMore") }}
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.gl-branches-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.gl-branches-head .grow {
  flex: 1;
}

.gl-branches-filter {
  display: flex;
  align-items: center;
  gap: 4px;
}

.gl-branches-filter .input {
  width: 220px;
}

.gl-branch-section + .gl-branch-section {
  border-top: 1px solid var(--border);
}

.gl-branch-section-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px 2px;
}

.gl-branch-section-title {
  margin: 0;
  font-size: 13px;
  font-weight: 650;
  color: var(--muted);
}

.gl-branch-none {
  padding: 12px 14px 16px;
  color: var(--muted);
  font-size: 12.5px;
}

.gl-branch-row {
  align-items: center;
  gap: 10px;
}

.gl-branch-icon {
  color: var(--muted);
  margin-top: 2px;
}

.gl-branch-line {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.gl-branch-name {
  font-size: 13.5px;
}

.gl-branch-row.clickable {
  cursor: pointer;
}

.gl-branch-name-btn {
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  cursor: pointer;
}

.gl-branch-name-btn:hover {
  color: var(--accent);
  text-decoration: underline;
}

.gl-branch-meta {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
}

/* 提交标题可能很长：优先截断标题，保住 sha 与时间。 */
.gl-branch-commit {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: min(520px, 42vw);
}

.gl-branch-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.gl-branch-more {
  padding: 2px 14px 12px;
}
</style>
