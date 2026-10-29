<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t } from "../i18n";
import { formatDuration, relativeTime } from "../lib/format";
import { aggregateStatus } from "../lib/status";
import CreatePipelineDialog from "./CreatePipelineDialog.vue";
import JobDrawer from "./JobDrawer.vue";
import StatusBadge from "./StatusBadge.vue";
import type { GitLabProject, Job, Pipeline, PipelineDetail, TestReport } from "../types";

const props = withDefaults(
  defineProps<{
    project: GitLabProject;
    readOnly?: boolean;
    initialPipelineId?: number | null;
    initialJobId?: number | null;
  }>(),
  { readOnly: true, initialPipelineId: null, initialJobId: null },
);
const emit = defineEmits<{
  navigate: [pipelineId: number | null];
  "select-job": [jobId: number | null];
  "open-commit": [sha: string];
}>();

const pipelines = ref<Pipeline[]>([]);
const page = ref(1);
const hasMore = ref(false);
const loading = ref(false);
const error = ref("");
const refFilter = ref(props.project.default_branch || "");
const statusFilter = ref("");

const selectedId = ref<number | null>(null);
const detail = ref<PipelineDetail | null>(null);
const jobs = ref<Job[]>([]);
const detailLoading = ref(false);
const detailError = ref("");
const selectedJob = ref<Job | null>(null);
const showCreate = ref(false);
const acting = ref(false);
const testReport = ref<TestReport | null>(null);

let timer: number | undefined;

const statuses = [
  { value: "", label: "stateAll" },
  { value: "running", label: "status_running" },
  { value: "success", label: "status_success" },
  { value: "failed", label: "status_failed" },
  { value: "canceled", label: "status_canceled" },
] as const;

const stages = computed(() => {
  const order: string[] = [];
  const buckets = new Map<string, Job[]>();
  for (const job of jobs.value) {
    if (!buckets.has(job.stage)) {
      buckets.set(job.stage, []);
      order.push(job.stage);
    }
    buckets.get(job.stage)?.push(job);
  }
  return order.map((name) => ({ name, jobs: buckets.get(name) ?? [] }));
});

const failedCases = computed(() =>
  (testReport.value?.test_suites ?? []).flatMap((suite) =>
    (suite.test_cases ?? []).filter((item) => item.status === "failed"),
  ),
);

function stageStatus(items: Job[]): string {
  return aggregateStatus(items.map((job) => job.status));
}

async function load(reset = true): Promise<void> {
  if (loading.value) return;
  loading.value = true;
  error.value = "";
  if (reset) page.value = 1;
  try {
    const result = await gitlabApi.pipelines({
      projectId: props.project.path_with_namespace,
      ref: refFilter.value.trim(),
      status: statusFilter.value,
      page: page.value,
      perPage: 20,
    });
    pipelines.value = reset ? result.items : [...pipelines.value, ...result.items];
    hasMore.value = result.has_more;
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

async function loadDetail(): Promise<void> {
  if (!selectedId.value) return;
  detailLoading.value = true;
  detailError.value = "";
  try {
    const [pipeline, jobPage, report] = await Promise.all([
      gitlabApi.pipeline(props.project.path_with_namespace, selectedId.value),
      gitlabApi.pipelineJobs(props.project.path_with_namespace, selectedId.value, 1, 100),
      gitlabApi
        .pipelineTestReport(props.project.path_with_namespace, selectedId.value)
        .catch(() => null),
    ]);
    detail.value = pipeline;
    jobs.value = jobPage.items;
    testReport.value = report;
    if (selectedJob.value) {
      selectedJob.value =
        jobs.value.find((job) => job.id === selectedJob.value?.id) ?? selectedJob.value;
    }
    if (!selectedJob.value && props.initialJobId) {
      selectedJob.value = jobs.value.find((job) => job.id === props.initialJobId) ?? null;
    }
  } catch (err) {
    detailError.value = describeError(err);
  } finally {
    detailLoading.value = false;
  }
}

async function openPipeline(id: number): Promise<void> {
  selectedId.value = id;
  selectedJob.value = null;
  detail.value = null;
  jobs.value = [];
  testReport.value = null;
  await loadDetail();
  schedule();
  emit("navigate", id);
}

function back(): void {
  selectedId.value = null;
  detail.value = null;
  jobs.value = [];
  selectedJob.value = null;
  window.clearInterval(timer);
  void load(true);
  emit("navigate", null);
  emit("select-job", null);
}

/** 点作业卡：右侧抽屉打开，jobId 进路由，深链可还原。 */
function selectJob(job: Job): void {
  selectedJob.value = job;
  emit("select-job", job.id);
}

function closeDrawer(): void {
  selectedJob.value = null;
  emit("select-job", null);
}

async function playJob(job: Job): Promise<void> {
  acting.value = true;
  detailError.value = "";
  try {
    await gitlabApi.playJob(props.project.path_with_namespace, job.id);
    await loadDetail();
    selectJob(jobs.value.find((item) => item.id === job.id) ?? job);
  } catch (err) {
    detailError.value = describeError(err);
  } finally {
    acting.value = false;
  }
}

function schedule(): void {
  window.clearInterval(timer);
  timer = window.setInterval(() => {
    if (document.hidden || !selectedId.value) return;
    const status = detail.value?.status ?? "";
    if (["running", "pending", "created", "preparing", "waiting_for_resource"].includes(status)) {
      void loadDetail();
    }
  }, 3000);
}

async function act(action: "retry" | "cancel"): Promise<void> {
  if (!selectedId.value) return;
  acting.value = true;
  detailError.value = "";
  try {
    if (action === "retry") {
      await gitlabApi.retryPipeline(props.project.path_with_namespace, selectedId.value);
    } else {
      await gitlabApi.cancelPipeline(props.project.path_with_namespace, selectedId.value);
    }
    await loadDetail();
  } catch (err) {
    detailError.value = describeError(err);
  } finally {
    acting.value = false;
  }
}

function onCreated(pipeline: Pipeline): void {
  showCreate.value = false;
  void openPipeline(pipeline.id);
}

onMounted(() => {
  if (props.initialPipelineId) {
    void openPipeline(props.initialPipelineId);
  } else {
    void load(true);
  }
});

watch(
  () => props.initialPipelineId,
  (id) => {
    if (id && id !== selectedId.value) {
      void openPipeline(id);
    } else if (!id && selectedId.value) {
      selectedId.value = null;
      detail.value = null;
      jobs.value = [];
      selectedJob.value = null;
      window.clearInterval(timer);
      void load(true);
    }
  },
);

watch(
  () => props.initialJobId,
  (id) => {
    if (id) {
      const found = jobs.value.find((job) => job.id === id);
      if (found) selectedJob.value = found;
    } else if (selectedJob.value) {
      selectedJob.value = null;
    }
  },
);

onBeforeUnmount(() => window.clearInterval(timer));
</script>

<template>
  <section class="page">
    <template v-if="!selectedId">
      <div class="toolbar">
        <input v-model="refFilter" class="input mono" type="text" :placeholder="t('pipelineRef')" @keydown.enter="load(true)" />
        <div class="tabs">
          <button
            v-for="option in statuses"
            :key="option.value"
            type="button"
            :class="{ active: statusFilter === option.value }"
            @click="statusFilter = option.value; load(true)"
          >
            {{ t(option.label) }}
          </button>
        </div>
        <button class="btn" type="button" :disabled="loading" @click="load(true)">
          {{ t("refresh") }}
        </button>
        <span class="grow"></span>
        <button
          class="btn primary"
          type="button"
          :disabled="readOnly"
          :title="readOnly ? t('readOnlyActionHint') : ''"
          @click="showCreate = true"
        >
          {{ t("runPipeline") }}
        </button>
      </div>

      <div v-if="error" class="error-banner">
        <span class="grow">{{ error }}</span>
        <button class="btn ghost" type="button" @click="load(true)">{{ t("retry") }}</button>
      </div>

      <div class="card">
        <div v-if="loading && !pipelines.length" class="loading">{{ t("loading") }}</div>
        <div v-else-if="!pipelines.length" class="empty">{{ t("noPipelines") }}</div>
        <div v-else class="list">
          <div
            v-for="pipeline in pipelines"
            :key="pipeline.id"
            class="list-item"
            role="button"
            tabindex="0"
            @click="openPipeline(pipeline.id)"
            @keydown.enter="openPipeline(pipeline.id)"
            @keydown.space.prevent="openPipeline(pipeline.id)"
          >
            <div>
              <div class="title">
                <span class="mono muted">#{{ pipeline.id }}</span>
                {{ pipeline.ref }}
              </div>
              <div class="sub">
                <button
                  class="link mono"
                  type="button"
                  :title="t('commit')"
                  @click.stop="emit('open-commit', pipeline.sha)"
                >
                  {{ pipeline.sha.slice(0, 8) }}
                </button>
                · {{ pipeline.source || "-" }}
                · {{ relativeTime(pipeline.updated_at, locale) }}
              </div>
            </div>
            <div class="right">
              <StatusBadge :status="pipeline.status" />
            </div>
          </div>
        </div>
      </div>

      <div v-if="hasMore" class="footer-note">
        <button class="btn" type="button" :disabled="loading" @click="page += 1; load(false)">
          {{ t("loadMore") }}
        </button>
      </div>
    </template>

    <template v-else>
      <div class="toolbar">
        <button class="btn ghost" type="button" @click="back">← {{ t("back") }}</button>
        <button class="btn" type="button" :disabled="detailLoading" @click="loadDetail">
          {{ t("refresh") }}
        </button>
        <span class="grow"></span>
        <button
          v-if="detail && (detail.status === 'failed' || detail.status === 'canceled')"
          class="btn"
          type="button"
          :disabled="readOnly || acting"
          @click="act('retry')"
        >
          {{ t("retryPipeline") }}
        </button>
        <button
          v-if="detail && ['running', 'pending', 'created', 'preparing'].includes(detail.status)"
          class="btn"
          type="button"
          :disabled="readOnly || acting"
          @click="act('cancel')"
        >
          {{ t("cancelPipeline") }}
        </button>
      </div>

      <div v-if="detailError" class="error-banner">
        <span class="grow">{{ detailError }}</span>
        <button class="btn ghost" type="button" @click="loadDetail">{{ t("retry") }}</button>
      </div>

      <div v-if="detailLoading && !detail" class="loading">{{ t("loading") }}</div>

      <template v-else-if="detail">
        <div class="card card-body">
          <div class="mr-meta">
            <span class="mono muted">#{{ detail.id }}</span>
            <StatusBadge :status="detail.status" />
            <span class="branch">{{ detail.ref }}</span>
            <button
              class="link mono small"
              type="button"
              :title="t('commit')"
              @click="emit('open-commit', detail.sha)"
            >
              {{ detail.sha.slice(0, 8) }}
            </button>
            <span>{{ t("duration") }}: {{ formatDuration(detail.duration) }}</span>
            <span>{{ relativeTime(detail.created_at, locale) }}</span>
          </div>
        </div>

        <div v-if="!jobs.length" class="card"><div class="empty">{{ t("noJobs") }}</div></div>

        <div v-if="testReport && testReport.total_count" class="card">
          <div class="card-head">
            <h2>{{ t("testReport") }}</h2>
            <span class="count">{{ testReport.total_count }}</span>
          </div>
          <div class="card-body">
            <div class="project-stats" style="border-top: 0; padding: 0">
              <div class="stat">
                <span>{{ t("testsPassed") }}</span>
                <strong>{{ testReport.success_count }}</strong>
              </div>
              <div class="stat">
                <span>{{ t("testsFailed") }}</span>
                <strong>{{ testReport.failed_count }}</strong>
              </div>
              <div class="stat">
                <span>{{ t("testsSkipped") }}</span>
                <strong>{{ testReport.skipped_count }}</strong>
              </div>
              <div class="stat">
                <span>{{ t("testDuration") }}</span>
                <strong>{{ testReport.total_time.toFixed(2) }}s</strong>
              </div>
            </div>

            <div v-if="failedCases.length" class="failed-cases">
              <h3>{{ t("failedTests") }}</h3>
              <div v-for="item in failedCases" :key="item.name + item.file" class="failed-case">
                <span class="mono">{{ item.name }}</span>
                <span class="muted small">
                  {{ item.file }}
                  <template v-if="item.execution_time">
                    · {{ item.execution_time.toFixed(2) }}s
                  </template>
                </span>
              </div>
            </div>
          </div>
        </div>

        <div v-if="jobs.length" class="pipeline-board">
          <section
            v-for="stage in stages"
            :key="stage.name"
            class="pipeline-stage"
            :data-status="stageStatus(stage.jobs)"
          >
            <header class="stage-head">
              <span class="stage-name">{{ stage.name }}</span>
              <StatusBadge :status="stageStatus(stage.jobs)" />
            </header>
            <div class="stage-jobs">
              <div
                v-for="job in stage.jobs"
                :key="job.id"
                class="job-card"
                :class="{ active: selectedJob?.id === job.id, failed: job.status === 'failed' }"
                role="button"
                tabindex="0"
                @click="selectJob(job)"
                @keydown.enter.prevent="selectJob(job)"
              >
                <StatusBadge :status="job.status" />
                <span class="mono job-name">{{ job.name }}</span>
                <span class="muted small">{{ formatDuration(job.duration) }}</span>
                <button
                  v-if="job.status === 'manual'"
                  class="job-play"
                  type="button"
                  :title="t('playJob')"
                  :disabled="readOnly || acting"
                  @click.stop="playJob(job)"
                >
                  ▶
                </button>
              </div>
            </div>
          </section>
        </div>

        <JobDrawer
          v-if="selectedJob"
          :project="project"
          :job="selectedJob"
          :read-only="readOnly"
          @close="closeDrawer"
          @changed="loadDetail"
        />
      </template>
    </template>

    <CreatePipelineDialog
      v-if="showCreate"
      :project="project"
      @close="showCreate = false"
      @created="onCreated"
    />
  </section>
</template>
