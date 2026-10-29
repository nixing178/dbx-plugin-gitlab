<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t } from "../i18n";
import { copyText } from "../lib/clipboard";
import { diffStats } from "../lib/diff";
import { commitBody, formatDateTime, relativeTime } from "../lib/format";
import DiffFileView from "./DiffFile.vue";
import Icon from "./Icon.vue";
import type { CommitDetail, DiffFile, MergeRequest } from "../types";

const props = defineProps<{ projectPath: string; sha: string }>();
const emit = defineEmits<{
  back: [];
  "open-commit": [sha: string];
  "open-mr": [mr: MergeRequest];
}>();

const detail = ref<CommitDetail | null>(null);
const diffs = ref<DiffFile[]>([]);
const loading = ref(true);
const diffLoading = ref(false);
const error = ref("");
const diffError = ref("");
const copied = ref(false);

const commit = computed(() => detail.value?.commit ?? null);
const body = computed(() =>
  commit.value ? commitBody(commit.value.message, commit.value.title) : "",
);
const parent = computed(() => commit.value?.parent_ids?.[0] ?? "");
const branches = computed(() => (detail.value?.refs ?? []).filter((ref) => ref.type === "branch"));
const tags = computed(() => (detail.value?.refs ?? []).filter((ref) => ref.type === "tag"));
const mergeRequests = computed(() => detail.value?.merge_requests ?? []);
const initial = computed(() => (commit.value?.author_name || "G").slice(0, 1).toUpperCase());
const shortSha = computed(() => (commit.value?.short_id || props.sha).slice(0, 8));

const stats = computed(() =>
  diffs.value.reduce(
    (total, file) => {
      const stat = diffStats(file.diff ?? "");
      return {
        additions: total.additions + stat.additions,
        deletions: total.deletions + stat.deletions,
      };
    },
    { additions: 0, deletions: 0 },
  ),
);

/** 提交本身失败才是整页失败；diff 失败只影响「变更」区。 */
async function loadDiff(): Promise<void> {
  diffLoading.value = true;
  diffError.value = "";
  try {
    diffs.value = await gitlabApi.commitDiff(props.projectPath, props.sha);
  } catch (err) {
    diffError.value = describeError(err);
  } finally {
    diffLoading.value = false;
  }
}

async function load(): Promise<void> {
  if (!props.projectPath || !props.sha) {
    error.value = t("projectMissing");
    loading.value = false;
    return;
  }

  loading.value = true;
  error.value = "";
  detail.value = null;
  diffs.value = [];
  diffError.value = "";

  const detailTask = gitlabApi
    .commitDetail(props.projectPath, props.sha)
    .then((value) => {
      detail.value = value;
    })
    .catch((err) => {
      error.value = describeError(err);
    });

  await Promise.all([detailTask, loadDiff()]);
  loading.value = false;
}

async function copySha(): Promise<void> {
  if (!props.sha) return;
  copied.value = await copyText(props.sha);
  if (copied.value) window.setTimeout(() => (copied.value = false), 1500);
}

watch(() => [props.projectPath, props.sha], () => void load(), { immediate: true });
</script>

<template>
  <section class="page commit-page">
    <div class="toolbar">
      <button class="btn ghost" type="button" @click="emit('back')">
        ← {{ t("backToCommits") }}
      </button>
      <button class="btn" type="button" :disabled="loading" @click="load">
        {{ t("refresh") }}
      </button>
      <span class="spacer"></span>
      <span class="muted small mono">{{ shortSha }}</span>
    </div>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="load">{{ t("retry") }}</button>
    </div>

    <div v-if="loading && !commit" class="loading">{{ t("loading") }}</div>

    <template v-else-if="commit">
      <div class="card card-body commit-head">
        <div class="commit-head-meta">
          <span class="mono">{{ commit.short_id }}</span>
          <span>{{ t("committedAgo", { time: relativeTime(commit.committed_date, locale) }) }}</span>
          <span class="commit-author">
            <span class="gl-avatar sm">{{ initial }}</span>
            {{ commit.author_name }}
          </span>
          <span class="spacer"></span>
          <button class="btn ghost small" type="button" @click="copySha">
            {{ copied ? t("copied") : t("copySha") }}
          </button>
          <a
            v-if="commit.web_url"
            class="btn ghost small"
            :href="commit.web_url"
            target="_blank"
            rel="noreferrer"
          >
            {{ t("openInGitLab") }}
            <Icon name="external" :size="13" />
          </a>
        </div>

        <h1 class="commit-title">{{ commit.title }}</h1>
        <pre v-if="body" class="commit-message">{{ body }}</pre>
      </div>

      <div class="card commit-facts">
        <div class="commit-fact">
          <span class="side-label">{{ t("parentCommit") }}</span>
          <button
            v-if="parent"
            class="link mono"
            type="button"
            @click="emit('open-commit', parent)"
          >
            {{ parent.slice(0, 8) }}
          </button>
          <span v-else class="muted">{{ t("rootCommit") }}</span>
        </div>

        <div class="commit-fact">
          <span class="side-label">{{ t("branches") }}</span>
          <div v-if="branches.length" class="label-list">
            <span
              v-for="ref in branches"
              :key="'branch-' + ref.name"
              class="badge"
              data-tone="neutral"
            >
              {{ ref.name }}
            </span>
          </div>
          <span v-else class="muted">{{ t("noBranches") }}</span>
        </div>

        <div class="commit-fact">
          <span class="side-label">{{ t("tags") }}</span>
          <div v-if="tags.length" class="label-list">
            <span v-for="ref in tags" :key="'tag-' + ref.name" class="badge" data-tone="neutral">
              {{ ref.name }}
            </span>
          </div>
          <span v-else class="muted">{{ t("noTags") }}</span>
        </div>

        <div class="commit-fact">
          <span class="side-label">{{ t("relatedMergeRequests") }}</span>
          <div v-if="mergeRequests.length" class="commit-mr-list">
            <button
              v-for="mr in mergeRequests"
              :key="mr.iid"
              class="link"
              type="button"
              @click="emit('open-mr', mr)"
            >
              <span class="mono">!{{ mr.iid }}</span> {{ mr.title }}
            </button>
          </div>
          <span v-else class="muted">{{ t("noRelatedMergeRequests") }}</span>
        </div>
      </div>

      <div class="card">
        <div class="commit-changes-head">
          <h2 class="timeline-title">
            {{ t("changes") }}
            <span class="muted">{{ diffs.length }}</span>
          </h2>
          <span class="diff-stat">
            <span class="add">+{{ stats.additions }}</span>
            <span class="del">-{{ stats.deletions }}</span>
          </span>
          <span class="spacer"></span>
          <span class="muted small">
            {{ t("committedAt") }} {{ formatDateTime(commit.committed_date, locale) }}
          </span>
        </div>

        <div v-if="diffError" class="gl-inline-warn">
          <span class="grow">{{ diffError }}</span>
          <button class="btn ghost small" type="button" @click="loadDiff">{{ t("retry") }}</button>
        </div>
        <p v-else-if="diffLoading" class="loading">{{ t("loading") }}</p>
        <p v-else-if="!diffs.length" class="empty">{{ t("diffEmpty") }}</p>
        <template v-else>
          <DiffFileView
            v-for="file in diffs"
            :key="file.old_path + ':' + file.new_path"
            :file="file"
            :project-path="projectPath"
            :git-ref="sha"
            :show-viewed="false"
          />
        </template>
      </div>
    </template>
  </section>
</template>
