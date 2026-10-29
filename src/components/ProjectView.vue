<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t, type MessageKey } from "../i18n";
import { copyText } from "../lib/clipboard";
import { relativeTime } from "../lib/format";
import type { ProjectView as ProjectViewName } from "../lib/nav";
import CommitHistory from "./CommitHistory.vue";
import CodeSearch from "./CodeSearch.vue";
import GitRefSelect from "./GitRefSelect.vue";
import Icon from "./Icon.vue";
import MemberList from "./MemberList.vue";
import ProjectInfo from "./ProjectInfo.vue";
import RepoBrowser from "./RepoBrowser.vue";
import type { Commit, GitLabProject, RepoOverview } from "../types";

const props = withDefaults(
  defineProps<{ project: GitLabProject; view: ProjectViewName; readOnly?: boolean }>(),
  { readOnly: false },
);
const emit = defineEmits<{
  "update:view": [value: ProjectViewName];
  "open-commit": [commit: Commit];
  "star-changed": [payload: { starred: boolean; starCount: number }];
}>();

const refName = ref(props.project.default_branch || "");
const overview = ref<RepoOverview | null>(null);
const overviewLoading = ref(false);
const overviewError = ref("");

const codeOpen = ref(false);
const copied = ref(false);

const starred = ref(Boolean(props.project.starred));
const starCount = ref(Number(props.project.star_count ?? 0));
const starring = ref(false);
const starError = ref("");
/** 用户手动点过星标后，异步查询的结果不再覆盖。 */
const starTouched = ref(false);

const views: Array<{ id: ProjectViewName; label: MessageKey; icon: string }> = [
  { id: "files", label: "files", icon: "folder" },
  { id: "commits", label: "commits", icon: "commit" },
  { id: "members", label: "members", icon: "users" },
  { id: "search", label: "codeSearch", icon: "search" },
];

const isFileView = computed(() => props.view === "overview" || props.view === "files");
const headCommit = computed(() => overview.value?.head_commit ?? null);
const initials = computed(() => (props.project.name || props.project.path).slice(0, 1).toUpperCase());

function authorInitial(name: string): string {
  return (name || "G").slice(0, 1).toUpperCase();
}

async function loadOverview(): Promise<void> {
  overviewLoading.value = true;
  overviewError.value = "";
  try {
    overview.value = await gitlabApi.repoOverview(
      props.project.path_with_namespace,
      refName.value,
    );
  } catch (err) {
    overviewError.value = describeError(err);
  } finally {
    overviewLoading.value = false;
  }
}

/** 星标状态单独查（项目详情不返回 starred），失败保持未加星，不阻塞页面。 */
async function syncStarred(): Promise<void> {
  try {
    const result = await gitlabApi.projectStarred(props.project.path_with_namespace);
    if (starTouched.value) return;
    starred.value = result.starred;
    if (result.starred) {
      emit("star-changed", { starred: true, starCount: starCount.value });
    }
  } catch {
    /* 实例不支持时保持未加星 */
  }
}

/** 加星/取消星标；304 时后端返回 star_count = -1，保持本地计数不动。 */
async function toggleStar(): Promise<void> {
  if (props.readOnly || starring.value) return;
  starTouched.value = true;
  const next = !starred.value;
  starring.value = true;
  starError.value = "";
  try {
    const result = await gitlabApi.starProject(props.project.path_with_namespace, next);
    starred.value = result.starred;
    if (result.star_count >= 0) starCount.value = result.star_count;
    emit("star-changed", { starred: starred.value, starCount: starCount.value });
  } catch (err) {
    starError.value = describeError(err);
  } finally {
    starring.value = false;
  }
}

function chooseBranch(name: string): void {
  if (!name || name === refName.value) return;
  refName.value = name;
  overview.value = null;
  void loadOverview();
}

function setView(view: ProjectViewName): void {
  codeOpen.value = false;
  emit("update:view", view);
}

async function copySha(): Promise<void> {
  const sha = headCommit.value?.id;
  if (!sha) return;
  copied.value = await copyText(sha);
  if (copied.value) window.setTimeout(() => (copied.value = false), 1500);
}

watch(
  () => props.project.path_with_namespace,
  () => {
    refName.value = props.project.default_branch || "";
    overview.value = null;
    starred.value = Boolean(props.project.starred);
    starCount.value = Number(props.project.star_count ?? 0);
    starError.value = "";
    starTouched.value = false;
    void loadOverview();
    void syncStarred();
  },
);

// 列表对象不带 starred，父级补完详情后同步按钮状态。
watch(
  () => [props.project.starred, props.project.star_count] as const,
  ([nextStarred, nextCount]) => {
    if (nextStarred === undefined) return;
    starred.value = Boolean(nextStarred);
    starCount.value = Number(nextCount ?? 0);
  },
);

watch(
  () => props.view,
  () => {
    codeOpen.value = false;
    if (!overview.value) void loadOverview();
  },
);

function onDocumentClick(event: MouseEvent): void {
  const target = event.target as HTMLElement | null;
  if (!target?.closest(".gl-dropdown")) {
    codeOpen.value = false;
  }
}

onMounted(() => {
  void loadOverview();
  void syncStarred();
  document.addEventListener("click", onDocumentClick);
});

onBeforeUnmount(() => document.removeEventListener("click", onDocumentClick));
</script>

<template>
  <section class="gl-project">
    <header class="gl-project-head">
      <div class="gl-project-title">
        <span class="gl-avatar">{{ initials }}</span>
        <h1>{{ project.name }}</h1>
        <Icon v-if="project.visibility === 'private'" name="lock" :size="14" />
        <span v-if="project.archived" class="badge" data-tone="warning">{{ t("archived") }}</span>
        <span v-if="!project.default_branch" class="badge" data-tone="neutral">
          {{ t("emptyRepository") }}
        </span>
      </div>

      <div class="gl-head-actions">
        <button
          class="gl-counter gl-star-toggle"
          :class="{ starred }"
          type="button"
          :disabled="readOnly || starring"
          :aria-pressed="starred"
          :title="readOnly ? t('readOnlyActionHint') : starred ? t('unstarProject') : t('starProject')"
          @click="toggleStar"
        >
          <Icon name="star" :size="14" />{{ starCount }}
        </button>
        <span v-if="starError" class="gl-star-error">{{ starError }}</span>
        <span class="gl-counter" :title="t('forks')">
          <Icon name="fork" :size="14" />{{ project.forks_count }}
        </span>
        <a
          class="gl-icon-btn"
          :href="project.web_url"
          target="_blank"
          rel="noreferrer"
          :title="t('openInGitLab')"
          :aria-label="t('openInGitLab')"
        >
          <Icon name="external" :size="15" />
        </a>
      </div>
    </header>

    <div class="gl-controls">
      <GitRefSelect
        class="gl-branch-select"
        :project-id="project.path_with_namespace"
        :model-value="refName || project.default_branch || ''"
        :placeholder="t('mrNewSelectBranch')"
        :clearable="false"
        @update:model-value="chooseBranch"
      />

      <span class="gl-controls-path mono">{{ project.path }} /</span>
      <span class="spacer"></span>

      <button class="gl-btn subtle" type="button" @click="setView('commits')">
        {{ t("history") }}
      </button>
      <button class="gl-btn subtle" type="button" @click="setView('search')">
        {{ t("findFile") }}
      </button>
      <div class="gl-dropdown">
        <button class="gl-btn subtle" type="button" @click="codeOpen = !codeOpen">
          {{ t("projectNavCode") }}
          <Icon name="chevron-down" :size="14" />
        </button>
        <div v-if="codeOpen" class="gl-dropdown-menu right">
          <button
            v-for="entry in views"
            :key="entry.id"
            type="button"
            class="gl-dropdown-item"
            :class="{ active: view === entry.id }"
            @click="setView(entry.id)"
          >
            <Icon :name="entry.icon" :size="14" />
            <span>{{ t(entry.label) }}</span>
          </button>
        </div>
      </div>
    </div>

    <div class="gl-project-body">
      <div class="gl-project-main">
        <template v-if="isFileView">
          <div v-if="overviewError" class="gl-inline-warn">{{ overviewError }}</div>

          <div v-if="headCommit" class="gl-commit-bar">
            <button
              class="gl-commit-open"
              type="button"
              :title="headCommit.title"
              @click="emit('open-commit', headCommit)"
            >
              <span class="gl-commit-avatar">{{ authorInitial(headCommit.author_name) }}</span>
              <div class="gl-commit-body">
                <div class="gl-commit-title">{{ headCommit.title }}</div>
                <div class="gl-commit-meta">
                  <span>{{ headCommit.author_name }}</span>
                  <span class="muted">
                    {{ t("committedAgo", { time: relativeTime(headCommit.committed_date, locale) }) }}
                  </span>
                </div>
              </div>
              <span class="gl-commit-sha mono">{{ headCommit.short_id }}</span>
            </button>
            <button
              class="gl-icon-btn"
              type="button"
              :title="t('copySha')"
              :aria-label="t('copySha')"
              @click="copySha"
            >
              {{ copied ? t("copied") : t("copy") }}
            </button>
          </div>
          <div v-else-if="overviewLoading" class="gl-commit-bar gl-commit-bar-skeleton">
            <span class="gl-commit-avatar">·</span>
            <div class="gl-commit-body">
              <span class="muted small">{{ t("loading") }}</span>
            </div>
          </div>

          <RepoBrowser
            :project="project"
            :ref-name="refName"
            :show-readme="view === 'overview'"
            @open-commit="(commit) => emit('open-commit', commit)"
          />
        </template>

        <CommitHistory
          v-else-if="view === 'commits'"
          :project="project"
          :ref-name="refName"
          @open="(commit) => emit('open-commit', commit)"
        />
        <MemberList v-else-if="view === 'members'" :project="project" />
        <CodeSearch v-else :project="project" />
      </div>

      <ProjectInfo :project="project" :overview="overview" @open-view="setView" />
    </div>
  </section>
</template>
