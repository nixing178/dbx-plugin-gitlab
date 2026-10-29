<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { describeError, gitlabApi } from "../api";
import { copyText } from "../lib/clipboard";
import { relativeTime } from "../lib/format";
import { locale, t } from "../i18n";
import {
  branchIssue,
  buildCreateMrPayload,
  canCompare,
  commitDraft,
  compareSummary,
  defaultMergeOptions,
  nextAutoValue,
} from "../lib/mrDraft";
import type {
  Branch,
  CompareResult,
  GitLabProject,
  GitLabUser,
  Label,
  Member,
  MergeRequest,
  Milestone,
} from "../types";
import GitRefSelect from "./GitRefSelect.vue";
import GitSelect from "./GitSelect.vue";
import Icon from "./Icon.vue";
import MarkdownEditor from "./MarkdownEditor.vue";

/**
 * GitLab「新建合并请求」整页表单。
 *
 * 两段式：先选源/目标分支并比较，比较成功后折叠分支区、展开表单；
 * 「更改分支」可退回第一段，已填内容不丢。
 */
const props = withDefaults(
  defineProps<{ project: GitLabProject; me?: GitLabUser | null; readOnly?: boolean }>(),
  { me: null, readOnly: true },
);
const emit = defineEmits<{
  created: [mr: MergeRequest];
  cancel: [];
  "open-commit": [sha: string];
}>();

const mergeDefaults = defaultMergeOptions();

const sourceBranch = ref("");
const targetBranch = ref(props.project.default_branch || "main");
const sourceHead = ref<Branch | null>(null);
const targetHead = ref<Branch | null>(null);

const compared = ref(false);
const comparing = ref(false);
const skipped = ref(false);
const compareResult = ref<CompareResult | null>(null);
const compareError = ref("");
const tab = ref<"commits" | "changes">("commits");

const title = ref("");
const description = ref("");
/** 上一次自动填入的值；重新比较时据此判断用户有没有手改。 */
const autoTitle = ref("");
const autoDescription = ref("");
const draft = ref(false);
const milestoneId = ref(0);
const labelNames = ref<string[]>([]);
const assigneeIds = ref<number[]>([]);
const reviewerIds = ref<number[]>([]);
const removeSourceBranch = ref(mergeDefaults.removeSourceBranch);
const squash = ref(mergeDefaults.squash);

const members = ref<Member[]>([]);
const labels = ref<Label[]>([]);
const milestones = ref<Milestone[]>([]);
const membersLoading = ref(false);
const labelsLoading = ref(false);
const milestonesLoading = ref(false);

const busy = ref(false);
const error = ref("");
const copied = ref("");

const projectId = computed(() => props.project.path_with_namespace);
const issue = computed(() => branchIssue(sourceBranch.value, targetBranch.value));
const issueText = computed(() => {
  switch (issue.value) {
    case "sourceMissing":
    case "targetMissing":
      return t("mrNewBranchRequired");
    case "sameBranch":
      return t("mrNewBranchSame");
    default:
      return "";
  }
});
const summary = computed(() => compareSummary(compareResult.value));
const compareReady = computed(() => canCompare(sourceBranch.value, targetBranch.value) && !comparing.value);
const canSubmit = computed(
  () => !issue.value && title.value.trim().length > 0 && !busy.value && !props.readOnly,
);

const peopleOptions = computed(() =>
  members.value.map((member) => ({
    value: String(member.id),
    label: member.name || member.username,
    hint: `@${member.username}`,
    avatar: (member.name || member.username).slice(0, 1).toUpperCase(),
  })),
);

const labelOptions = computed(() =>
  labels.value.map((label) => ({
    value: label.name,
    label: label.name,
    hint: label.description,
    color: label.color,
    textColor: label.text_color,
  })),
);

const milestoneOptions = computed(() =>
  milestones.value.map((milestone) => ({
    value: String(milestone.id),
    label: milestone.title,
    hint: milestone.due_date ? `Due ${milestone.due_date}` : undefined,
  })),
);

const assigneeValues = computed(() => assigneeIds.value.map(String));
const reviewerValues = computed(() => reviewerIds.value.map(String));
const milestoneValue = computed(() => (milestoneId.value > 0 ? String(milestoneId.value) : ""));

/** 单项加载失败只留空，不挡住整页。 */
async function loadMeta(): Promise<void> {
  membersLoading.value = true;
  labelsLoading.value = true;
  milestonesLoading.value = true;

  void gitlabApi
    .projectMembers(projectId.value, "", 1, 100)
    .then((page) => (members.value = page.items))
    .catch(() => (members.value = []))
    .finally(() => (membersLoading.value = false));

  void gitlabApi
    .labels(projectId.value)
    .then((page) => (labels.value = page.items))
    .catch(() => (labels.value = []))
    .finally(() => (labelsLoading.value = false));

  void gitlabApi
    .milestones(projectId.value)
    .then((page) => (milestones.value = page.items))
    .catch(() => (milestones.value = []))
    .finally(() => (milestonesLoading.value = false));

  // 目标分支默认取项目默认分支，先把它的最新提交补上，别让右卡空着。
  void gitlabApi
    .branches(projectId.value, targetBranch.value, 1, 20)
    .then((page) => {
      targetHead.value = page.items.find((item) => item.name === targetBranch.value) ?? null;
    })
    .catch(() => {
      targetHead.value = null;
    });
}

/** 用最早的一条提交预填标题/描述；用户手改过的字段保持原样。 */
function applyCommitDraft(result: CompareResult | null): void {
  const draft = commitDraft(result?.commits);
  title.value = nextAutoValue(title.value, autoTitle.value, draft.title);
  autoTitle.value = draft.title;
  description.value = nextAutoValue(description.value, autoDescription.value, draft.description);
  autoDescription.value = draft.description;
}

async function runCompare(): Promise<void> {
  if (!compareReady.value) return;
  comparing.value = true;
  compareError.value = "";
  try {
    const result = await gitlabApi.compare(
      projectId.value,
      targetBranch.value.trim(),
      sourceBranch.value.trim(),
    );
    compareResult.value = result;
    compared.value = true;
    skipped.value = false;
    applyCommitDraft(result);
    tab.value = result.compare_timeout ? "changes" : "commits";
  } catch (err) {
    compareError.value = describeError(err);
  } finally {
    comparing.value = false;
  }
}

/** 比较接口不可用（老实例 / 403）时的显式降级，不是默认路径。 */
function skipCompare(): void {
  compareResult.value = null;
  compared.value = true;
  skipped.value = true;
  compareError.value = "";
}

function changeBranches(): void {
  compared.value = false;
  skipped.value = false;
  compareResult.value = null;
  compareError.value = "";
}

function onBranchSelect(side: "source" | "target", branch: Branch | null): void {
  if (side === "source") {
    sourceHead.value = branch;
  } else {
    targetHead.value = branch;
  }
}

function assignToMe(): void {
  if (!props.me) return;
  assigneeIds.value = [props.me.id];
}

function onMilestoneChange(value: string): void {
  milestoneId.value = Number(value) || 0;
}

function onAssigneeChange(values: string[]): void {
  assigneeIds.value = values.map(Number).filter((id) => id > 0);
}

function onReviewerChange(values: string[]): void {
  reviewerIds.value = values.map(Number).filter((id) => id > 0);
}

async function copySha(sha: string): Promise<void> {
  if (!sha) return;
  copied.value = (await copyText(sha)) ? sha : "";
  window.setTimeout(() => {
    if (copied.value === sha) copied.value = "";
  }, 2000);
}

async function submit(): Promise<void> {
  if (!canSubmit.value) return;
  busy.value = true;
  error.value = "";
  try {
    const mr = await gitlabApi.createMr(
      buildCreateMrPayload({
        projectId: projectId.value,
        sourceBranch: sourceBranch.value,
        targetBranch: targetBranch.value,
        title: title.value,
        description: description.value,
        draft: draft.value,
        milestoneId: milestoneId.value,
        labels: labelNames.value,
        assigneeIds: assigneeIds.value,
        reviewerIds: reviewerIds.value,
        removeSourceBranch: removeSourceBranch.value,
        squash: squash.value,
      }),
    );
    emit("created", mr);
  } catch (err) {
    error.value = describeError(err);
  } finally {
    busy.value = false;
  }
}

function authorName(name: string): string {
  return name || "-";
}

onMounted(() => void loadMeta());
</script>

<template>
  <section class="page gl-new-mr">
    <header class="gl-new-mr-head">
      <h1>{{ t("mrNewHeading") }}</h1>
      <p v-if="compared" class="gl-new-mr-subtitle">
        {{ t("mrNewFrom") }}
        <span class="mono gl-new-mr-ref">{{ sourceBranch }}</span>
        {{ t("mrNewInto") }}
        <span class="mono gl-new-mr-ref">{{ targetBranch }}</span>
        <button class="link-btn" type="button" @click="changeBranches">
          {{ t("mrNewChangeBranches") }}
        </button>
      </p>
    </header>

    <p v-if="readOnly" class="gl-inline-warn">{{ t("readOnlyActionHint") }}</p>
    <p v-if="error" class="error-text">{{ error }}</p>

    <div v-if="!compared" class="gl-new-mr-branches">
      <div class="gl-new-mr-branch">
        <h3>{{ t("sourceBranch") }}</h3>
        <div class="gl-new-mr-pickers">
          <span class="gl-new-mr-project" :title="projectId">{{ projectId }}</span>
          <GitRefSelect
            :project-id="projectId"
            :model-value="sourceBranch"
            :placeholder="t('mrNewSelectBranch')"
            @update:model-value="sourceBranch = $event"
            @select="onBranchSelect('source', $event)"
          />
        </div>

        <div class="gl-new-mr-commit">
          <template v-if="sourceHead?.commit">
            <button
              class="gl-new-mr-commit-open"
              type="button"
              :title="sourceHead.commit.title"
              @click="emit('open-commit', sourceHead.commit.id)"
            >
              <span class="gl-new-mr-dot" data-state="ready"></span>
              <span class="gl-new-mr-commit-body">
                <span class="gl-new-mr-commit-title">{{ sourceHead.commit.title }}</span>
                <span class="gl-new-mr-commit-meta">
                  {{ authorName(sourceHead.commit.author_name) }} ·
                  {{ relativeTime(sourceHead.commit.authored_date, locale) }}
                </span>
              </span>
              <span class="gl-new-mr-sha mono">{{ sourceHead.commit.short_id }}</span>
            </button>
            <button
              class="gl-icon-btn"
              type="button"
              :title="t('copySha')"
              :aria-label="t('copySha')"
              @click="copySha(sourceHead.commit.id)"
            >
              <Icon :name="copied === sourceHead.commit.id ? 'check' : 'disk'" :size="14" />
            </button>
          </template>
          <template v-else>
            <span class="gl-new-mr-dot"></span>
            <span class="muted">{{ t("mrNewSelectBranchHint") }}</span>
          </template>
        </div>
      </div>

      <div class="gl-new-mr-branch">
        <h3>{{ t("targetBranch") }}</h3>
        <div class="gl-new-mr-pickers">
          <span class="gl-new-mr-project" :title="projectId">{{ projectId }}</span>
          <GitRefSelect
            :project-id="projectId"
            :model-value="targetBranch"
            :placeholder="t('mrNewSelectBranch')"
            @update:model-value="targetBranch = $event"
            @select="onBranchSelect('target', $event)"
          />
        </div>

        <div class="gl-new-mr-commit">
          <template v-if="targetHead?.commit">
            <button
              class="gl-new-mr-commit-open"
              type="button"
              :title="targetHead.commit.title"
              @click="emit('open-commit', targetHead.commit.id)"
            >
              <span class="gl-new-mr-dot" data-state="ready"></span>
              <span class="gl-new-mr-commit-body">
                <span class="gl-new-mr-commit-title">{{ targetHead.commit.title }}</span>
                <span class="gl-new-mr-commit-meta">
                  {{ authorName(targetHead.commit.author_name) }} ·
                  {{ relativeTime(targetHead.commit.authored_date, locale) }}
                </span>
              </span>
              <span class="gl-new-mr-sha mono">{{ targetHead.commit.short_id }}</span>
            </button>
            <button
              class="gl-icon-btn"
              type="button"
              :title="t('copySha')"
              :aria-label="t('copySha')"
              @click="copySha(targetHead.commit.id)"
            >
              <Icon :name="copied === targetHead.commit.id ? 'check' : 'disk'" :size="14" />
            </button>
          </template>
          <template v-else>
            <span class="gl-new-mr-dot"></span>
            <span class="muted">{{ t("mrNewSelectBranchHint") }}</span>
          </template>
        </div>
      </div>
    </div>

    <div v-if="!compared" class="gl-new-mr-compare">
      <button class="btn primary" type="button" :disabled="!compareReady" @click="runCompare">
        {{ comparing ? t("loading") : t("mrNewCompare") }}
      </button>
      <span v-if="issueText" class="muted">{{ issueText }}</span>
    </div>

    <div v-if="compareError" class="error-banner">
      <span class="grow">{{ compareError }}</span>
      <button class="btn" type="button" @click="runCompare">{{ t("retry") }}</button>
      <button class="btn ghost" type="button" @click="skipCompare">{{ t("mrNewSkipCompare") }}</button>
    </div>

    <template v-if="compared">
      <div class="gl-new-mr-field">
        <label for="mr-new-title">{{ t("mrNewTitleLabel") }}</label>
        <input id="mr-new-title" v-model="title" class="input" type="text" />
      </div>

      <label class="checkbox">
        <input v-model="draft" type="checkbox" />
        {{ t("draft") }}
      </label>
      <p class="hint">{{ t("mrNewDraftHint") }}</p>

      <div class="gl-new-mr-field">
        <label>{{ t("mrDescription") }}</label>
        <MarkdownEditor v-model="description" />
      </div>

      <div class="gl-new-mr-meta">
        <div class="gl-new-mr-meta-row">
          <span class="gl-new-mr-meta-label">{{ t("mrNewAssignee") }}</span>
          <div class="gl-new-mr-meta-control">
            <GitSelect
              mode="multiple"
              :placeholder="t('mrNewUnassigned')"
              :options="peopleOptions"
              :values="assigneeValues"
              :loading="membersLoading"
              :empty-text="t('mrNewNoPeople')"
              @update:values="onAssigneeChange"
            />
            <button class="link-btn" type="button" :disabled="!me" @click="assignToMe">
              {{ t("mrNewAssignToMe") }}
            </button>
          </div>
        </div>

        <div class="gl-new-mr-meta-row">
          <span class="gl-new-mr-meta-label">{{ t("reviewers") }}</span>
          <div class="gl-new-mr-meta-control">
            <GitSelect
              mode="multiple"
              :placeholder="t('mrNewUnassigned')"
              :options="peopleOptions"
              :values="reviewerValues"
              :loading="membersLoading"
              :empty-text="t('mrNewNoPeople')"
              @update:values="onReviewerChange"
            />
          </div>
        </div>

        <div class="gl-new-mr-meta-row">
          <span class="gl-new-mr-meta-label">{{ t("milestone") }}</span>
          <div class="gl-new-mr-meta-control">
            <GitSelect
              :placeholder="t('mrNewNoMilestone')"
              :options="milestoneOptions"
              :value="milestoneValue"
              :loading="milestonesLoading"
              :empty-text="t('mrNewEmptyMilestones')"
              @update:value="onMilestoneChange"
            />
          </div>
        </div>

        <div class="gl-new-mr-meta-row">
          <span class="gl-new-mr-meta-label">{{ t("labels") }}</span>
          <div class="gl-new-mr-meta-control">
            <GitSelect
              mode="multiple"
              :placeholder="t('mrNewNoLabels')"
              :options="labelOptions"
              :values="labelNames"
              :loading="labelsLoading"
              :empty-text="t('mrNewEmptyLabels')"
              @update:values="labelNames = $event"
            />
          </div>
        </div>
      </div>

      <fieldset class="gl-new-mr-merge">
        <legend>{{ t("mrNewMergeOptions") }}</legend>
        <label class="checkbox">
          <input v-model="removeSourceBranch" type="checkbox" />
          {{ t("mrNewDeleteSource") }}
        </label>
        <label class="checkbox">
          <input v-model="squash" type="checkbox" />
          {{ t("mrNewSquash") }}
        </label>
      </fieldset>

      <div class="gl-new-mr-submit">
        <button class="btn primary" type="button" :disabled="!canSubmit" @click="submit">
          {{ busy ? t("loading") : t("createMr") }}
        </button>
        <button class="btn" type="button" :disabled="busy" @click="emit('cancel')">
          {{ t("cancel") }}
        </button>
      </div>

      <p v-if="skipped" class="gl-inline-warn">{{ t("mrNewSkippedCompare") }}</p>

      <template v-else>
      <div class="gl-new-mr-tabs" role="tablist">
        <button
          type="button"
          role="tab"
          :class="{ active: tab === 'commits' }"
          :aria-selected="tab === 'commits'"
          @click="tab = 'commits'"
        >
          {{ t("commits") }}
          <span class="gl-tab-count">{{ summary.commits }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :class="{ active: tab === 'changes' }"
          :aria-selected="tab === 'changes'"
          @click="tab = 'changes'"
        >
          {{ t("changes") }}
          <span class="gl-tab-count">{{ summary.changes }}</span>
        </button>
      </div>

      <p v-if="summary.timedOut" class="gl-inline-warn">{{ t("mrNewCompareTimeout") }}</p>
      <p v-else-if="summary.sameRef" class="gl-inline-warn">{{ t("mrNewSameRef") }}</p>

      <div v-if="tab === 'commits'" class="gl-new-mr-commits">
        <p v-if="!summary.commits" class="muted">{{ t("mrNewNoCommits") }}</p>
        <div v-for="commit in compareResult?.commits ?? []" :key="commit.id" class="gl-new-mr-commit-row">
          <button
            class="gl-new-mr-commit-open"
            type="button"
            :title="commit.title"
            @click="emit('open-commit', commit.id)"
          >
            <span class="gl-avatar sm">{{ authorName(commit.author_name).slice(0, 2).toUpperCase() }}</span>
            <span class="gl-new-mr-commit-body">
              <span class="gl-new-mr-commit-title">{{ commit.title }}</span>
              <span class="gl-new-mr-commit-meta">
                {{ authorName(commit.author_name) }} ·
                {{ relativeTime(commit.authored_date, locale) }}
              </span>
            </span>
            <span class="gl-new-mr-sha mono">{{ commit.short_id }}</span>
          </button>
          <button
            class="gl-icon-btn"
            type="button"
            :title="t('copySha')"
            :aria-label="t('copySha')"
            @click="copySha(commit.id)"
          >
            <Icon :name="copied === commit.id ? 'check' : 'disk'" :size="14" />
          </button>
        </div>
      </div>

      <div v-else class="gl-new-mr-changes">
        <p v-if="!summary.changes" class="muted">{{ t('mrNewNoChanges') }}</p>
        <div
          v-for="file in compareResult?.diffs ?? []"
          :key="`${file.old_path}->${file.new_path}`"
          class="gl-new-mr-change-row"
        >
          <span class="mono gl-ellipsis">{{ file.new_path || file.old_path }}</span>
          <span class="gl-new-mr-diffstat">
            <span class="gl-add">+{{ file.additions }}</span>
            <span class="gl-del">-{{ file.deletions }}</span>
          </span>
        </div>
      </div>
      </template>
    </template>
  </section>
</template>
