<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t } from "../i18n";
import { copyText } from "../lib/clipboard";
import {
  commitAuthorOptions,
  commitAvatarTone,
  filterCommits,
  groupCommitsByDay,
  type CommitAuthorCount,
} from "../lib/commitList";
import { relativeTime } from "../lib/format";
import Icon from "./Icon.vue";
import type { Commit, GitLabProject } from "../types";

const props = withDefaults(
  defineProps<{
    project: GitLabProject;
    refName?: string;
    /** 工具行里的作者筛选；空串表示全部作者。 */
    author?: string;
    /** 工具行里的消息关键字；组件内做本地过滤，不额外请求。 */
    query?: string;
  }>(),
  { refName: "", author: "", query: "" },
);
const emit = defineEmits<{
  open: [commit: Commit];
  /** 作者候选随列表刷新回抛给工具行，避免父组件重复聚合。 */
  authors: [options: CommitAuthorCount[]];
}>();

const commits = ref<Commit[]>([]);
const page = ref(1);
const hasMore = ref(false);
const loading = ref(false);
const error = ref("");
const copiedId = ref("");
let copyTimer = 0;

const filterActive = computed(() => Boolean(props.author.trim() || props.query.trim()));

/** 分组与筛选都在本地完成：筛选只作用于已加载的提交，翻页后继续生效。 */
const groups = computed(() =>
  groupCommitsByDay(
    filterCommits(commits.value, { author: props.author, query: props.query }),
    locale.value,
  ),
);

/** 筛选无命中与「分支没有提交」是两种状态，文案要区分。 */
const emptyText = computed(() => (filterActive.value ? t("noMatchingCommits") : t("noData")));

function authorInitial(name: string): string {
  return (name || "G").trim().slice(0, 1).toUpperCase();
}

function commitTime(commit: Commit): string {
  return relativeTime(commit.committed_date || commit.authored_date, locale.value);
}

async function load(reset = true): Promise<void> {
  // 换分支时允许打断进行中的请求；「加载更多」才需要防重入。
  if (loading.value && !reset) return;
  loading.value = true;
  error.value = "";
  if (reset) page.value = 1;
  try {
    const result = await gitlabApi.commits(
      props.project.path_with_namespace,
      props.refName || props.project.default_branch,
      page.value,
      30,
    );
    commits.value = reset ? result.items : [...commits.value, ...result.items];
    hasMore.value = result.has_more;
    emit("authors", commitAuthorOptions(commits.value));
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

/** 复制按钮在行内，不能让点击冒泡到行按钮上（会跳详情）。 */
async function copySha(commit: Commit): Promise<void> {
  const ok = await copyText(commit.id);
  if (!ok) return;
  copiedId.value = commit.id;
  window.clearTimeout(copyTimer);
  copyTimer = window.setTimeout(() => (copiedId.value = ""), 1500);
}

/** 切换分支只改列表数据源，不重置当前详情页（详情页由路由承载）。 */
watch(
  () => [props.project.path_with_namespace, props.refName],
  () => void load(true),
);

onMounted(() => void load(true));

defineExpose({ reload: () => load(true) });
</script>

<template>
  <div class="gl-commit-list">
    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="load(true)">{{ t("retry") }}</button>
    </div>

    <div v-if="filterActive && hasMore" class="gl-commit-hint">
      {{ t("commitFilterHint", { count: commits.length }) }}
    </div>

    <div v-if="loading && !commits.length" class="loading">{{ t("loading") }}</div>
    <div v-else-if="!groups.length" class="empty">{{ emptyText }}</div>
    <template v-else>
      <section v-for="group in groups" :key="group.key || 'unknown'" class="gl-commit-day">
        <h2 v-if="group.label" class="gl-commit-day-label">{{ group.label }}</h2>
        <div class="gl-commit-rows">
          <div v-for="commit in group.commits" :key="commit.id" class="gl-commit-row">
            <button class="gl-commit-row-main" type="button" @click="emit('open', commit)">
              <span class="gl-commit-avatar" :data-tone="commitAvatarTone(commit.author_name)">
                {{ authorInitial(commit.author_name) }}
              </span>
              <span class="gl-commit-row-text">
                <span class="gl-commit-row-title" :title="commit.title">{{ commit.title }}</span>
                <span class="gl-commit-row-meta">
                  {{
                    t("commitAuthorMeta", {
                      author: commit.author_name,
                      time: commitTime(commit),
                    })
                  }}
                </span>
              </span>
            </button>
            <span class="gl-commit-row-actions">
              <span class="gl-commit-sha mono" :title="commit.id">{{ commit.short_id }}</span>
              <button
                class="gl-commit-copy"
                :class="{ copied: copiedId === commit.id }"
                type="button"
                :title="copiedId === commit.id ? t('copied') : t('copySha')"
                :aria-label="t('copySha')"
                @click.stop="copySha(commit)"
              >
                <Icon :name="copiedId === commit.id ? 'check' : 'copy'" :size="14" />
              </button>
            </span>
          </div>
        </div>
      </section>
    </template>

    <div v-if="hasMore" class="gl-commit-more">
      <button class="btn" type="button" :disabled="loading" @click="page += 1; load(false)">
        {{ t("loadMore") }}
      </button>
    </div>
  </div>
</template>
