<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import type { MessageKey } from "../i18n";
import { locale, t } from "../i18n";
import { relativeTime } from "../lib/format";
import {
  MR_ORDER_BYS,
  mrCountLabel,
  mrCountsQuery,
  mrEmptyCopy,
  mrListState,
  mrQuery,
  mrSort,
  mrTabs,
  type MrFilterInput,
  type MrState,
} from "../lib/mrFilters";
import EmptyState from "./EmptyState.vue";
import Icon from "./Icon.vue";
import StatusBadge from "./StatusBadge.vue";
import type { MrScope } from "../lib/nav";
import type { GitLabProject, MergeRequest, MrCounts } from "../types";

const props = withDefaults(
  defineProps<{
    project: GitLabProject | null;
    readOnly?: boolean;
    scope?: MrScope;
    state?: string;
    search?: string;
    orderBy?: string;
    sort?: string;
  }>(),
  { readOnly: true, scope: "project", state: "opened", search: "", orderBy: "", sort: "" },
);
const emit = defineEmits<{
  open: [mr: MergeRequest];
  create: [];
  "update:scope": [value: MrScope];
  "update:state": [value: string];
  "update:search": [value: string];
  "update:order-by": [value: string];
  "update:sort": [value: string];
}>();

const STATE_LABELS: Record<MrState, MessageKey> = {
  opened: "stateOpened",
  merged: "stateMerged",
  closed: "stateClosed",
  all: "stateAll",
};

const ORDER_LABELS: Record<string, MessageKey> = {
  updated_at: "mrSortUpdated",
  created_at: "mrSortCreated",
};

/** 全局视角才需要 scope；项目内固定按项目过滤（对齐 GitLab 项目 MR 页）。 */
const globalScopes: Array<{ value: MrScope; label: MessageKey }> = [
  { value: "reviewer", label: "scopeReviewer" },
  { value: "created_by_me", label: "scopeCreated" },
  { value: "assigned_to_me", label: "scopeAssigned" },
];

const items = ref<MergeRequest[]>([]);
const counts = ref<MrCounts | null>(null);
const page = ref(1);
const hasMore = ref(false);
const loading = ref(false);
const error = ref("");
const searchDraft = ref(props.search);
let debounce: number | undefined;

const projectPath = computed(() => props.project?.path_with_namespace ?? "");
const scoped = computed(() => Boolean(projectPath.value));
const currentState = computed(() => mrListState(props.state));
const tabs = computed(() => mrTabs(counts.value));
const sortState = computed(() => mrSort(props.orderBy, props.sort));
const emptyCopy = computed(() => mrEmptyCopy(props.state, props.search));
const emptyTitle = computed(() => t(emptyCopy.value.titleKey as MessageKey));
const emptyHint = computed(() => t(emptyCopy.value.hintKey as MessageKey));

const filterInput = computed<MrFilterInput>(() => ({
  project: projectPath.value,
  scope: props.scope,
  state: props.state,
  search: props.search,
  orderBy: props.orderBy,
  sort: props.sort,
}));

function stateLabel(state: MrState): string {
  return t(STATE_LABELS[state]);
}

function orderLabel(orderBy: string): string {
  return t(ORDER_LABELS[orderBy] ?? "mrSortUpdated");
}

function authorName(mr: MergeRequest): string {
  return mr.author?.name || mr.author?.username || "-";
}

async function load(reset = true): Promise<void> {
  if (loading.value) return;
  loading.value = true;
  error.value = "";
  if (reset) page.value = 1;

  try {
    const query = mrQuery(filterInput.value);
    const result = await gitlabApi.mergeRequests({
      ...query,
      page: page.value,
      perPage: 30,
    });
    items.value = reset ? result.items : [...items.value, ...result.items];
    hasMore.value = result.has_more;
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

/** 计数失败只隐藏页签数字，绝不能挡住列表。 */
async function loadCounts(): Promise<void> {
  try {
    counts.value = await gitlabApi.mrCounts(mrCountsQuery(filterInput.value));
  } catch {
    counts.value = null;
  }
}

function refreshAll(): void {
  void load(true);
  void loadCounts();
}

function onSearchInput(): void {
  window.clearTimeout(debounce);
  debounce = window.setTimeout(() => emit("update:search", searchDraft.value.trim()), 300);
}

function onScopeChange(event: Event): void {
  emit("update:scope", (event.target as HTMLSelectElement).value as MrScope);
}

function onOrderChange(event: Event): void {
  emit("update:order-by", (event.target as HTMLSelectElement).value);
}

function toggleSort(): void {
  emit("update:sort", sortState.value.sort === "asc" ? "desc" : "asc");
}

watch(
  () => [props.scope, props.state, props.search, props.orderBy, props.sort],
  () => void load(true),
);
watch(
  () => [projectPath.value, props.scope, props.search],
  () => void loadCounts(),
  { immediate: true },
);
watch(
  () => props.search,
  (value) => {
    searchDraft.value = value;
  },
);
onMounted(() => void load(true));
onBeforeUnmount(() => window.clearTimeout(debounce));
</script>

<template>
  <section class="page">
    <div class="gl-mr-head">
      <nav class="project-tabs gl-mr-tabs" role="navigation">
        <button
          v-for="tab in tabs"
          :key="tab.state"
          type="button"
          :class="{ active: currentState === tab.state }"
          :aria-current="currentState === tab.state ? 'page' : undefined"
          @click="emit('update:state', tab.state)"
        >
          {{ stateLabel(tab.state) }}
          <span v-if="mrCountLabel(tab.count)" class="gl-tab-count">
            {{ mrCountLabel(tab.count) }}
          </span>
        </button>
      </nav>
      <button
        v-if="scoped"
        class="btn primary"
        type="button"
        :disabled="readOnly"
        :title="readOnly ? t('readOnlyActionHint') : ''"
        @click="emit('create')"
      >
        {{ t("createMr") }}
      </button>
      <button
        class="gl-icon-btn"
        type="button"
        :title="t('refresh')"
        :aria-label="t('refresh')"
        :disabled="loading"
        @click="refreshAll"
      >
        <Icon name="refresh" :size="16" />
      </button>
    </div>

    <div class="gl-mr-filters">
      <label v-if="!scoped" class="gl-scope-field">
        <Icon name="funnel" :size="14" />
        <select :value="scope" :aria-label="t('scopeProject')" @change="onScopeChange">
          <option v-for="option in globalScopes" :key="option.value" :value="option.value">
            {{ t(option.label) }}
          </option>
        </select>
      </label>
      <label class="search-field gl-mr-search">
        <Icon name="search" :size="15" />
        <input
          v-model="searchDraft"
          type="search"
          :placeholder="t('mrFilterSearchPlaceholder')"
          @input="onSearchInput"
          @keydown.enter="emit('update:search', searchDraft.trim())"
        />
      </label>
      <div class="gl-mr-sort">
        <select
          class="select"
          :value="sortState.orderBy"
          :aria-label="t('mrSortOrder')"
          @change="onOrderChange"
        >
          <option v-for="orderBy in MR_ORDER_BYS" :key="orderBy" :value="orderBy">
            {{ orderLabel(orderBy) }}
          </option>
        </select>
        <button
          class="gl-icon-btn"
          type="button"
          :title="sortState.sort === 'asc' ? t('mrSortAsc') : t('mrSortDesc')"
          :aria-label="sortState.sort === 'asc' ? t('mrSortAsc') : t('mrSortDesc')"
          @click="toggleSort"
        >
          <Icon name="sort" :size="15" :class="{ 'gl-sort-asc': sortState.sort === 'asc' }" />
        </button>
      </div>
    </div>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="refreshAll">{{ t("retry") }}</button>
    </div>

    <div class="card gl-mr-card">
      <div v-if="loading && !items.length" class="loading">{{ t("loading") }}</div>
      <EmptyState v-else-if="!items.length" :title="emptyTitle" :hint="emptyHint">
        <button v-if="scoped && !readOnly" class="btn primary" type="button" @click="emit('create')">
          {{ t("createMr") }}
        </button>
      </EmptyState>
      <div v-else class="list gl-mr-list">
        <button
          v-for="mr in items"
          :key="mr.id"
          class="list-item gl-mr-item"
          type="button"
          @click="emit('open', mr)"
        >
          <span class="gl-mr-state" :data-state="mr.state">
            <Icon name="merge" :size="16" />
          </span>
          <span class="gl-mr-main">
            <span class="gl-mr-title">
              <span class="mono muted">!{{ mr.iid }}</span>
              {{ mr.title }}
            </span>
            <span class="gl-mr-meta">
              {{ authorName(mr) }}
              ·
              <span class="mono">{{ mr.source_branch }}</span>
              →
              <span class="mono">{{ mr.target_branch }}</span>
              · {{ relativeTime(mr.updated_at, locale) }}
            </span>
          </span>
          <span class="gl-mr-badges">
            <span v-if="mr.draft" class="badge" data-tone="neutral">{{ t("draft") }}</span>
            <StatusBadge v-if="mr.pipeline" :status="mr.pipeline.status" />
            <StatusBadge :status="mr.state" />
          </span>
        </button>
      </div>
    </div>

    <div v-if="hasMore" class="footer-note">
      <button class="btn" type="button" :disabled="loading" @click="page += 1; load(false)">
        {{ t("loadMore") }}
      </button>
    </div>
  </section>
</template>
