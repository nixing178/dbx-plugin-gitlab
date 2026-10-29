<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { describeError, gitlabApi } from "../api";
import { t } from "../i18n";
import { formatDuration } from "../lib/format";
import { countProblems, filterLines, toLines } from "../lib/logLines";
import StatusBadge from "./StatusBadge.vue";
import type { GitLabProject, Job } from "../types";

const props = withDefaults(
  defineProps<{ project: GitLabProject; job: Job; readOnly?: boolean }>(),
  { readOnly: true },
);
const emit = defineEmits<{ changed: [] }>();

const text = ref("");
const offset = ref(0);
const complete = ref(false);
const loading = ref(false);
const error = ref("");
const follow = ref(true);
const filter = ref<"all" | "problems">("all");
const query = ref("");
const copied = ref(false);
const acting = ref(false);
const artifactsBusy = ref(false);
const logEl = ref<HTMLElement | null>(null);

let timer: number | undefined;

const lines = computed(() => toLines(text.value));
const shown = computed(() => filterLines(lines.value, filter.value, query.value));
const problems = computed(() => countProblems(lines.value));
const live = computed(
  () => !complete.value || props.job.status === "running" || props.job.status === "pending",
);

async function scrollBottom(): Promise<void> {
  await nextTick();
  const element = logEl.value;
  if (element) element.scrollTop = element.scrollHeight;
}

async function pull(): Promise<void> {
  if (loading.value) return;
  loading.value = true;
  try {
    const chunk = await gitlabApi.jobTrace(
      props.project.path_with_namespace,
      props.job.id,
      offset.value,
    );
    if (chunk.text) text.value += chunk.text;
    offset.value = chunk.next_offset;
    complete.value = chunk.complete;
    error.value = "";
    if (follow.value) await scrollBottom();
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

function schedule(): void {
  window.clearInterval(timer);
  timer = window.setInterval(() => {
    if (document.hidden) return;
    if (live.value) void pull();
  }, 1500);
}

async function act(action: "retry" | "cancel" | "play"): Promise<void> {
  acting.value = true;
  error.value = "";
  try {
    const projectId = props.project.path_with_namespace;
    if (action === "retry") await gitlabApi.retryJob(projectId, props.job.id);
    else if (action === "cancel") await gitlabApi.cancelJob(projectId, props.job.id);
    else await gitlabApi.playJob(projectId, props.job.id);
    emit("changed");
  } catch (err) {
    error.value = describeError(err);
  } finally {
    acting.value = false;
  }
}

async function copyLog(): Promise<void> {
  try {
    if (window.dbxPlugin?.copy) await window.dbxPlugin.copy(text.value);
    else await navigator.clipboard.writeText(text.value);
    copied.value = true;
    window.setTimeout(() => (copied.value = false), 1500);
  } catch {
    // 剪贴板不可用时静默失败，用户仍可下载日志。
  }
}

function download(): void {
  const blob = new Blob([text.value], { type: "text/plain;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `job-${props.job.id}.log`;
  link.click();
  URL.revokeObjectURL(url);
}

async function downloadArtifacts(): Promise<void> {
  artifactsBusy.value = true;
  error.value = "";
  try {
    const artifact = await gitlabApi.jobArtifacts(
      props.project.path_with_namespace,
      props.job.id,
    );
    if (artifact.too_large) {
      error.value = t("artifactsTooLarge");
      return;
    }

    const binary = atob(artifact.data);
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    const blob = new Blob([bytes], { type: "application/zip" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = artifact.filename || `job-${props.job.id}-artifacts.zip`;
    link.click();
    URL.revokeObjectURL(url);
  } catch (err) {
    error.value = describeError(err);
  } finally {
    artifactsBusy.value = false;
  }
}

onMounted(() => {
  void pull();
  schedule();
});

onBeforeUnmount(() => window.clearInterval(timer));
</script>

<template>
  <section class="card job-log">
    <header class="card-head">
      <strong class="mono">{{ job.name }}</strong>
      <StatusBadge :status="job.status" />
      <span class="muted small">{{ formatDuration(job.duration) }}</span>
      <span class="badge" :data-tone="live ? 'running' : 'neutral'">
        {{ live ? t("logLive") : t("logFinished") }}
      </span>
      <span class="grow"></span>
      <button class="btn ghost small" type="button" @click="copyLog">
        {{ copied ? t("logCopied") : t("copyLog") }}
      </button>
      <button class="btn ghost small" type="button" @click="download">{{ t("downloadLog") }}</button>
      <button
        v-if="job.artifacts_file?.size"
        class="btn ghost small"
        type="button"
        :disabled="artifactsBusy"
        @click="downloadArtifacts"
      >
        {{ artifactsBusy ? t("loading") : t("downloadArtifacts") }}
      </button>
      <button
        v-if="job.status === 'running' || job.status === 'pending'"
        class="btn small"
        type="button"
        :disabled="readOnly || acting"
        @click="act('cancel')"
      >
        {{ t("cancelJob") }}
      </button>
      <button
        v-if="job.status === 'failed' || job.status === 'canceled'"
        class="btn small"
        type="button"
        :disabled="readOnly || acting"
        @click="act('retry')"
      >
        {{ t("retryJob") }}
      </button>
      <button
        v-if="job.status === 'manual'"
        class="btn primary small"
        type="button"
        :disabled="readOnly || acting"
        @click="act('play')"
      >
        {{ t("playJob") }}
      </button>
    </header>

    <div class="log-toolbar">
      <div class="tabs">
        <button type="button" :class="{ active: filter === 'all' }" @click="filter = 'all'">
          {{ t("allLines") }}
        </button>
        <button type="button" :class="{ active: filter === 'problems' }" @click="filter = 'problems'">
          {{ t("problemsOnly") }}
          <span v-if="problems.errors + problems.warnings" class="badge" data-tone="danger">
            {{ problems.errors + problems.warnings }}
          </span>
        </button>
      </div>
      <input v-model="query" class="input grow" type="search" :placeholder="t('searchLog')" />
      <label class="checkbox">
        <input v-model="follow" type="checkbox" />
        {{ t("follow") }}
      </label>
    </div>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="pull">{{ t("retry") }}</button>
    </div>

    <div ref="logEl" class="log-body">
      <div
        v-for="line in shown"
        :key="line.number"
        class="log-row"
        :data-level="line.level"
      >
        <span class="ln">{{ line.number }}</span>
        <span class="msg">{{ line.text }}</span>
      </div>
      <p v-if="!shown.length" class="empty">
        {{ loading ? t("loading") : t("logEmpty") }}
      </p>
    </div>
  </section>
</template>
