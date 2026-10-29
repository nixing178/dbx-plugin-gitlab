<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t, type MessageKey } from "../i18n";
import { copyText } from "../lib/clipboard";
import { DIFF_PAGE_SIZE, diffInitialPages } from "../lib/diffPaging";
import { formatDateTime, relativeTime } from "../lib/format";
import { reactionSummary, type ReactionSummary } from "../lib/reactions";
import { statusKey, statusTone } from "../lib/status";
import type { MrTab } from "../lib/nav";
import { viewedScope } from "../lib/viewed";
import CommentComposer from "./CommentComposer.vue";
import ConfirmAction from "./ConfirmAction.vue";
import DiffBrowser from "./DiffBrowser.vue";
import DiscussionList from "./DiscussionList.vue";
import GitSelect from "./GitSelect.vue";
import Icon from "./Icon.vue";
import MarkdownBlock from "./MarkdownBlock.vue";
import MarkdownEditor from "./MarkdownEditor.vue";
import StatusBadge from "./StatusBadge.vue";
import type {
  AwardEmoji,
  CommentPosition,
  Commit,
  DiffFile,
  DiffRefs,
  Discussion,
  GitLabProject,
  GitLabUser,
  MergeRequest,
  MRApprovals,
} from "../types";

const props = withDefaults(
  defineProps<{
    project: GitLabProject;
    iid: number;
    readOnly?: boolean;
    tab?: MrTab;
    me?: GitLabUser | null;
  }>(),
  { readOnly: true, tab: "overview", me: null },
);
const emit = defineEmits<{
  back: [];
  "update:tab": [value: MrTab];
  "open-pipeline": [pipelineId: number];
  "open-commit": [sha: string];
}>();

type ConfirmKind = "" | "merge" | "close" | "approve" | "revoke";
type EditField = "" | "assignees" | "reviewers" | "labels" | "milestone";
type ActivityFilter = "all" | "comments";



interface SelectOption {
  value: string;
  label: string;
  hint?: string;
  color?: string;
  avatar?: string;
}

const mr = ref<MergeRequest | null>(null);
const diffs = ref<DiffFile[]>([]);
const diffRefs = ref<DiffRefs | null>(null);
const discussions = ref<Discussion[]>([]);
const commits = ref<Commit[]>([]);
const loading = ref(true);
const error = ref("");

const actionBusy = ref(false);
const actionError = ref("");
const confirmKind = ref<ConfirmKind>("");
const mergeSquash = ref(false);
const mergeRemoveSource = ref(true);
const readyOpen = ref(true);

const commentBusy = ref(false);
const commentDraft = ref("");
const activityFilter = ref<ActivityFilter>("all");
const inlinePosition = ref<CommentPosition | null>(null);

const editing = ref(false);
const editTitle = ref("");
const editDescription = ref("");
const saveBusy = ref(false);

const editField = ref<EditField>("");
const fieldBusy = ref(false);
const draftAssignees = ref<string[]>([]);
const draftReviewers = ref<string[]>([]);
const draftLabels = ref<string[]>([]);
const draftMilestone = ref("");
const members = ref<SelectOption[]>([]);
const labels = ref<SelectOption[]>([]);
const milestones = ref<SelectOption[]>([]);
const optionsLoading = ref(false);
const optionsLoaded = ref(false);
const milestonesLoaded = ref(false);

const approvals = ref<MRApprovals | null>(null);
const awards = ref<AwardEmoji[]>([]);
const reactionBusy = ref("");
const approvalsOpen = ref(true);

const codeMenu = ref<HTMLDetailsElement | null>(null);
const moreMenu = ref<HTMLDetailsElement | null>(null);
const toast = ref("");

const commitsLoading = ref(false);
const commitsError = ref("");
const commitsTotal = ref(0);

const diffsLoading = ref(false);
const diffsError = ref("");
const diffDone = ref(false);
const nextDiffPage = ref(1);
/** 连单文件都拉不动、被跳过的文件数（提示条里说明）。 */
const skippedDiffFiles = ref(0);

const scope = computed(() => viewedScope(props.project.path_with_namespace, props.iid));
const headSha = computed(() => diffRefs.value?.head_sha || mr.value?.sha || "head");
const pipeline = computed(() => mr.value?.pipeline ?? null);
const stateTone = computed(() => statusTone(mr.value?.state ?? ""));
const stateLabel = computed(() => t(statusKey(mr.value?.state ?? "") as MessageKey));

const authorName = computed(
  () => mr.value?.author?.name || mr.value?.author?.username || "-",
);

/** 参与者由作者 + 讨论区发言人推导，避免为了一个数字再打一次 participants 接口。 */
const participants = computed<GitLabUser[]>(() => {
  const seen = new Map<number, GitLabUser>();
  if (mr.value?.author) seen.set(mr.value.author.id, mr.value.author);
  for (const discussion of discussions.value) {
    for (const note of discussion.notes) {
      if (note.author) seen.set(note.author.id, note.author);
    }
  }
  return [...seen.values()];
});

const meId = computed(() => props.me?.id ?? 0);

const approvers = computed<GitLabUser[]>(() =>
  (approvals.value?.approved_by ?? [])
    .map((voter) => voter.user)
    .filter((user): user is GitLabUser => Boolean(user)),
);

const myApproval = computed(() => approvers.value.some((user) => user.id === meId.value));

/** 免费版没有审批规则时 approvals_required 为 0，显示「核准为可选」而不是「还差 0 个」。 */
const approvalHint = computed(() => {
  const required = approvals.value?.approvals_required ?? 0;
  if (required <= 0) return t("approvalOptional");
  const left = approvals.value?.approvals_left ?? required;
  return left > 0 ? t("approvalsNeeded", { count: left }) : t("approvalsDone");
});

const reactions = computed(() => reactionSummary(awards.value, meId.value));

/** GitLab 概览页签的数字是「未解决讨论数」，不是评论总数。 */
const unresolvedCount = computed(
  () =>
    discussions.value.filter((discussion) =>
      discussion.notes.some((note) => note.resolvable && !note.resolved),
    ).length,
);

/** 后端把「响应过大 / 格式异常」分成两类错误，这里都翻译成可读提示。 */
const diffErrorHint = computed(() => {
  if (!diffsError.value) return "";
  if (/timed out|timeout|interrupt/i.test(diffsError.value)) return t("diffLoadTimeout");
  if (/too large|unexpected response format/i.test(diffsError.value)) return t("diffLoadFailed");
  return diffsError.value;
});

/** 变更数用 MR 详情里的 changes_count（例如 36）；diff 只是分页明细，可能没拉全。 */
const changesCount = computed(() => {
  const raw = Number.parseInt(mr.value?.changes_count ?? "", 10);
  return Number.isFinite(raw) && raw > 0 ? raw : diffs.value.length;
});

const tabs = computed<Array<{ id: MrTab; label: string; count: number }>>(() => [
  { id: "overview", label: t("overview"), count: unresolvedCount.value },
  { id: "commits", label: t("tabCommits"), count: commitsTotal.value },
  { id: "pipelines", label: t("tabPipelines"), count: mr.value?.pipeline ? 1 : 0 },
  { id: "changes", label: t("tabChanges"), count: changesCount.value },
]);

/** 合并前的硬性阻塞条件：冲突、草稿、只读。 */
const mergeBlockedReason = computed(() => {
  if (!mr.value) return "";
  if (props.readOnly) return t("readOnlyActionHint");
  if (mr.value.has_conflicts) return t("mergeBlockedConflict");
  if (mr.value.draft) return t("mergeBlockedDraft");
  return "";
});
const canMerge = computed(() => Boolean(mr.value) && !mergeBlockedReason.value);

const memberOptions = computed(() => members.value);
const labelOptions = computed(() => labels.value);

const confirmMeta = computed(() => {
  switch (confirmKind.value) {
    case "merge":
      return { title: t("mergeTitle"), detail: t("mergeDetail"), label: t("merge"), danger: true };
    case "close":
      return { title: t("closeTitle"), detail: "", label: t("closeMr"), danger: true };
    case "approve":
      return { title: t("approveTitle"), detail: "", label: t("approve"), danger: false };
    case "revoke":
      return { title: t("revokeTitle"), detail: "", label: t("revoke"), danger: false };
    default:
      return null;
  }
});

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const [detail, threads, approvalState, emoji] = await Promise.all([
      gitlabApi.mergeRequest(props.project.path_with_namespace, props.iid),
      gitlabApi
        .mrDiscussions(props.project.path_with_namespace, props.iid)
        .catch(() => [] as Discussion[]),
      // 审批与反应是附加信息：老实例 / 权限不足时降级，不影响详情主体。
      gitlabApi.mrApprovals(props.project.path_with_namespace, props.iid).catch(() => null),
      gitlabApi
        .mrAwardEmoji(props.project.path_with_namespace, props.iid)
        .catch(() => [] as AwardEmoji[]),
    ]);
    mr.value = detail;
    // 详情里带 diff_refs；/diffs 若返回数组就不会有，用它兜底行内评论定位。
    diffRefs.value = detail.diff_refs ?? null;
    discussions.value = threads;
    approvals.value = approvalState;
    awards.value = emoji;
    // diff 单独按页拉：单个大 MR 的差异可能几十 MB，一次拉完会被 8MB 上限截断。
    void loadDiffs();
    void loadCommitCount();
    if (props.tab === "commits") void loadCommits();
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

const DIFF_MAX_PAGES = 60;

function diffKey(file: DiffFile): string {
  return `${file.old_path}:${file.new_path}`;
}

function mergeDiffFiles(files: DiffFile[], collected: DiffFile[], seen: Set<string>): void {
  for (const file of files) {
    const key = diffKey(file);
    if (seen.has(key)) continue;
    seen.add(key);
    collected.push(file);
  }
}

/** 拉一页并合并进 collected；返回 false 表示已经到底。 */
async function fetchDiffPage(
  page: number,
  collected: DiffFile[],
  seen: Set<string>,
): Promise<boolean> {
  const result = await gitlabApi.mrDiffs(
    props.project.path_with_namespace,
    props.iid,
    page,
    DIFF_PAGE_SIZE,
  );
  diffRefs.value = diffRefs.value ?? result.diff_refs;
  mergeDiffFiles(result.items, collected, seen);
  // 实例没给 X-Next-Page 时按「满页」继续翻，避免只看到第一页的几个文件。
  if (!result.items.length) return false;
  if (!result.has_more && result.items.length < DIFF_PAGE_SIZE) return false;
  return result.has_more;
}

/**
 * 整页拉不动时的兜底：按 per_page=1 逐个文件拉。
 * 单个文件仍然失败（体积超过服务端/客户端上限）就跳过它，继续下一个；
 * 返回 true 表示这一页之后已经没有更多文件。
 */
async function rescueDiffPage(
  page: number,
  collected: DiffFile[],
  seen: Set<string>,
): Promise<boolean> {
  const startIndex = (page - 1) * DIFF_PAGE_SIZE + 1;
  const total = changesCount.value;
  for (let offset = 0; offset < DIFF_PAGE_SIZE; offset += 1) {
    const index = startIndex + offset;
    if (total > 0 && index > total) return true;
    try {
      const result = await gitlabApi.mrDiffs(
        props.project.path_with_namespace,
        props.iid,
        index,
        1,
      );
      if (!result.items.length) return true;
      diffRefs.value = diffRefs.value ?? result.diff_refs;
      mergeDiffFiles(result.items, collected, seen);
    } catch {
      skippedDiffFiles.value += 1;
    }
  }
  return false;
}

/**
 * 分页加载 diff。
 *
 * 大 MR 的差异可能几十 MB：按 5 个文件一页拉、首屏只拉够阈值，其余留给
 * 「展开所有文件」。某一页失败时跳过它继续拉后面的页，已加载的文件照常展示，
 * 只有「连续几页都失败且一个文件都没拿到」才算整页失败。
 */
async function loadDiffs(reset = true): Promise<void> {
  if (diffsLoading.value) return;
  if (reset) {
    diffsError.value = "";
    diffDone.value = false;
    nextDiffPage.value = 1;
    skippedDiffFiles.value = 0;
  }
  diffsLoading.value = true;

  const collected: DiffFile[] = reset ? [] : [...diffs.value];
  const seen = new Set(collected.map(diffKey));
  const maxPages = reset ? diffInitialPages(changesCount.value) : DIFF_MAX_PAGES;
  let page = nextDiffPage.value;
  let pages = 0;
  let done = false;
  let lastError = "";

  // 首屏若一页都没成功，强制多试两页，避免第一页体积过大就整页空白。
  while ((pages < maxPages || (!collected.length && pages < 3)) && page <= DIFF_MAX_PAGES) {
    try {
      const more = await fetchDiffPage(page, collected, seen);
      pages += 1;
      if (!more) {
        done = true;
        break;
      }
      page += 1;
    } catch (err) {
      pages += 1;
      lastError = describeError(err);
      // 整页失败 → 逐文件回捞这一页；单个大文件只跳过它自己。
      const reachedEnd = await rescueDiffPage(page, collected, seen);
      if (reachedEnd) {
        done = true;
        break;
      }
      page += 1;
    }
  }

  if (!collected.length && lastError) diffsError.value = lastError;
  diffs.value = collected;
  diffDone.value = done;
  nextDiffPage.value = done ? 1 : page;
  diffsLoading.value = false;
}

/** 提示条上的「展开所有文件」：从上次停下的页继续拉。 */
async function loadMoreDiffs(): Promise<void> {
  if (diffDone.value) return;
  await loadDiffs(false);
}

/** 概览页也要显示正确的提交数，这里只取 1 条读 X-Total。 */
async function loadCommitCount(): Promise<void> {
  try {
    const page = await gitlabApi.mrCommits(props.project.path_with_namespace, props.iid, 1, 1);
    commitsTotal.value = page.total || page.items.length;
  } catch {
    // 计数失败保留 0，不影响详情主体。
  }
}

/** MR 范围内的提交（GitLab 的页签数字也是这个口径）。 */
async function loadCommits(): Promise<void> {
  if (!mr.value || commitsLoading.value) return;
  commitsLoading.value = true;
  commitsError.value = "";
  try {
    const page = await gitlabApi.mrCommits(props.project.path_with_namespace, props.iid, 1, 30);
    commits.value = page.items;
    commitsTotal.value = page.total || page.items.length;
  } catch (err) {
    commitsError.value = describeError(err);
  } finally {
    commitsLoading.value = false;
  }
}

async function withAction(run: () => Promise<void>): Promise<void> {
  actionBusy.value = true;
  actionError.value = "";
  try {
    await run();
    await load();
    confirmKind.value = "";
    closeMenus();
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    actionBusy.value = false;
  }
}

/** 合并面板本身就是确认界面，勾选项即时可见，因此不再弹二次确认。 */
function mergeNow(): void {
  if (!canMerge.value) return;
  void withAction(async () => {
    await gitlabApi.mergeMr({
      projectId: props.project.path_with_namespace,
      iid: props.iid,
      sha: mr.value?.sha,
      squash: mergeSquash.value,
      removeSourceBranch: mergeRemoveSource.value,
    });
  });
}

function runConfirm(): void {
  const kind = confirmKind.value;
  if (!kind) return;
  void withAction(async () => {
    if (kind === "close") {
      await gitlabApi.setMrState(props.project.path_with_namespace, props.iid, "close");
    } else if (kind === "approve") {
      await gitlabApi.approveMr(props.project.path_with_namespace, props.iid, mr.value?.sha ?? "");
    } else if (kind === "revoke") {
      await gitlabApi.unapproveMr(props.project.path_with_namespace, props.iid);
    }
  });
}

function reopen(): void {
  void withAction(() =>
    gitlabApi
      .setMrState(props.project.path_with_namespace, props.iid, "reopen")
      .then(() => undefined),
  );
}

function approveNow(): void {
  void withAction(async () => {
    await gitlabApi.approveMr(props.project.path_with_namespace, props.iid, mr.value?.sha ?? "");
  });
}

function revokeNow(): void {
  void withAction(async () => {
    await gitlabApi.unapproveMr(props.project.path_with_namespace, props.iid);
  });
}

async function toggleReaction(state: ReactionSummary): Promise<void> {
  if (props.readOnly || reactionBusy.value) return;
  reactionBusy.value = state.name;
  actionError.value = "";
  try {
    if (state.mine && state.myId) {
      await gitlabApi.removeMrAwardEmoji(
        props.project.path_with_namespace,
        props.iid,
        state.myId,
      );
    } else {
      await gitlabApi.addMrAwardEmoji(props.project.path_with_namespace, props.iid, state.name);
    }
    awards.value = await gitlabApi.mrAwardEmoji(props.project.path_with_namespace, props.iid);
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    reactionBusy.value = "";
  }
}

/** 草稿开关：GitLab 用标题前缀表达，来回切换不叠加也不会漏删。 */
function toggleDraft(): void {
  if (!mr.value || props.readOnly) return;
  const base = mr.value.title.replace(/^\s*draft:\s*/i, "");
  const title = mr.value.draft ? base : `Draft: ${base}`;
  void withAction(async () => {
    await gitlabApi.updateMr({ projectId: props.project.path_with_namespace, iid: props.iid, title });
  });
}

async function submitComment(body: string): Promise<void> {
  commentBusy.value = true;
  actionError.value = "";
  try {
    await gitlabApi.commentMr({
      projectId: props.project.path_with_namespace,
      iid: props.iid,
      body,
    });
    discussions.value = await gitlabApi.mrDiscussions(
      props.project.path_with_namespace,
      props.iid,
    );
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    commentBusy.value = false;
  }
}

async function submitDraft(): Promise<void> {
  const body = commentDraft.value.trim();
  if (!body || commentBusy.value) return;
  await submitComment(body);
  if (!actionError.value) commentDraft.value = "";
}

async function submitInline(body: string): Promise<void> {
  if (!inlinePosition.value) return;
  commentBusy.value = true;
  actionError.value = "";
  try {
    await gitlabApi.commentMr({
      projectId: props.project.path_with_namespace,
      iid: props.iid,
      body,
      position: inlinePosition.value,
    });
    discussions.value = await gitlabApi.mrDiscussions(
      props.project.path_with_namespace,
      props.iid,
    );
    inlinePosition.value = null;
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    commentBusy.value = false;
  }
}

async function replyThread(discussionId: string, body: string): Promise<void> {
  actionError.value = "";
  try {
    await gitlabApi.replyDiscussion({
      projectId: props.project.path_with_namespace,
      iid: props.iid,
      discussionId,
      body,
    });
    discussions.value = await gitlabApi.mrDiscussions(
      props.project.path_with_namespace,
      props.iid,
    );
  } catch (err) {
    actionError.value = describeError(err);
  }
}

async function resolveThread(discussionId: string, resolved: boolean): Promise<void> {
  actionError.value = "";
  try {
    await gitlabApi.resolveDiscussion({
      projectId: props.project.path_with_namespace,
      iid: props.iid,
      discussionId,
      resolved,
    });
    discussions.value = await gitlabApi.mrDiscussions(
      props.project.path_with_namespace,
      props.iid,
    );
  } catch (err) {
    actionError.value = describeError(err);
  }
}

function startEdit(): void {
  editTitle.value = mr.value?.title ?? "";
  editDescription.value = mr.value?.description ?? "";
  editing.value = true;
}

async function saveEdit(): Promise<void> {
  saveBusy.value = true;
  actionError.value = "";
  try {
    await gitlabApi.updateMr({
      projectId: props.project.path_with_namespace,
      iid: props.iid,
      title: editTitle.value.trim(),
      description: editDescription.value,
    });
    editing.value = false;
    await load();
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    saveBusy.value = false;
  }
}

function initialOf(user: GitLabUser): string {
  return (user.name || user.username || "?").slice(0, 1).toUpperCase();
}

async function ensureOptions(): Promise<void> {
  if (optionsLoaded.value || optionsLoading.value) return;
  optionsLoading.value = true;
  try {
    const [people, tags] = await Promise.all([
      gitlabApi.projectMembers(props.project.path_with_namespace, "", 1, 100),
      gitlabApi.labels(props.project.path_with_namespace, "", 1, 100).catch(() => null),
    ]);
    members.value = people.items.map((user) => ({
      value: String(user.id),
      label: user.name || user.username,
      hint: user.username,
      avatar: (user.name || user.username || "?").slice(0, 1).toUpperCase(),
    }));
    labels.value = (tags?.items ?? []).map((label) => ({
      value: label.name,
      label: label.name,
      color: label.color,
    }));
    optionsLoaded.value = true;
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    optionsLoading.value = false;
  }
}

function startField(field: EditField): void {
  if (!mr.value) return;
  editField.value = field;
  if (field === "assignees") draftAssignees.value = mr.value.assignees.map((u) => String(u.id));
  if (field === "reviewers") draftReviewers.value = mr.value.reviewers.map((u) => String(u.id));
  if (field === "labels") draftLabels.value = [...mr.value.labels];
  if (field === "milestone") {
    draftMilestone.value = mr.value.milestone ? String(mr.value.milestone.id) : "";
    void ensureMilestones();
    return;
  }
  void ensureOptions();
}

async function ensureMilestones(): Promise<void> {
  if (milestonesLoaded.value || optionsLoading.value) return;
  optionsLoading.value = true;
  try {
    const page = await gitlabApi.milestones(
      props.project.path_with_namespace,
      "active",
      1,
      100,
    );
    milestones.value = page.items.map((item) => ({ value: String(item.id), label: item.title }));
    milestonesLoaded.value = true;
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    optionsLoading.value = false;
  }
}

async function saveField(): Promise<void> {
  const field = editField.value;
  if (!mr.value || !field) return;
  const params: {
    projectId: string;
    iid: number;
    assigneeIds?: number[];
    reviewerIds?: number[];
    labels?: string[];
    milestoneId?: number;
  } = { projectId: props.project.path_with_namespace, iid: props.iid };
  if (field === "assignees") params.assigneeIds = draftAssignees.value.map(Number);
  if (field === "reviewers") params.reviewerIds = draftReviewers.value.map(Number);
  if (field === "labels") params.labels = [...draftLabels.value];
  // 空选择传 0，GitLab 用它清空里程碑。
  if (field === "milestone") {
    params.milestoneId = draftMilestone.value ? Number(draftMilestone.value) : 0;
  }

  fieldBusy.value = true;
  actionError.value = "";
  try {
    await gitlabApi.updateMr(params);
    editField.value = "";
    await load();
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    fieldBusy.value = false;
  }
}

function closeMenus(): void {
  if (codeMenu.value) codeMenu.value.open = false;
  if (moreMenu.value) moreMenu.value.open = false;
}

function flash(message: string): void {
  toast.value = message;
  window.setTimeout(() => {
    if (toast.value === message) toast.value = "";
  }, 1800);
}

async function copySourceBranch(): Promise<void> {
  if (!mr.value) return;
  closeMenus();
  await copyText(mr.value.source_branch);
  flash(t("copied"));
}

async function copyLink(): Promise<void> {
  if (!mr.value) return;
  closeMenus();
  const url = mr.value.web_url || `${props.project.web_url}/-/merge_requests/${props.iid}`;
  await copyText(url);
  flash(t("linkCopied"));
}

function openMenuAction(kind: ConfirmKind | "reopen"): void {
  closeMenus();
  if (kind === "reopen") {
    reopen();
    return;
  }
  confirmKind.value = kind;
}

watch(
  () => props.tab,
  (tab) => {
    if (tab === "commits" && !commits.value.length) void loadCommits();
  },
);

onMounted(() => void load());
</script>

<template>
  <section class="page gl-mr-view">
    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="emit('back')">{{ t("back") }}</button>
      <button class="btn ghost" type="button" @click="load">{{ t("retry") }}</button>
    </div>
    <div v-if="actionError" class="error-banner">
      <span class="grow">{{ actionError }}</span>
      <button class="btn ghost" type="button" @click="actionError = ''">×</button>
    </div>

    <div v-if="loading && !mr" class="loading">{{ t("loading") }}</div>

    <template v-else-if="mr">
      <header class="gl-mr-view-head">
        <div class="gl-mr-view-title">
          <h1>{{ mr.title }}</h1>
          <div class="gl-mr-view-sub">
            <span class="gl-state-pill" :data-tone="stateTone">
              <Icon name="merge" :size="13" />
              {{ stateLabel }}
            </span>
            <span v-if="mr.draft" class="badge" data-tone="warning">{{ t("draft") }}</span>
            <span v-if="mr.has_conflicts" class="badge" data-tone="danger">
              {{ t("conflicts") }}
            </span>
            <span class="gl-mr-view-actor">
              <strong>{{ authorName }}</strong>
              {{ t("mrRequestedToMerge", { source: mr.source_branch, target: mr.target_branch }) }}
            </span>
            <span class="muted">· {{ relativeTime(mr.created_at, locale) }}</span>
          </div>
        </div>

        <div class="gl-mr-view-actions">
          <span v-if="toast" class="muted small gl-mr-toast">{{ toast }}</span>
          <button
            v-if="mr.state !== 'merged'"
            class="gl-btn"
            type="button"
            :disabled="readOnly"
            :title="readOnly ? t('readOnlyActionHint') : ''"
            @click="startEdit"
          >
            {{ t("edit") }}
          </button>

          <details ref="codeMenu" class="gl-menu">
            <summary class="gl-btn">
              {{ t("projectNavCode") }}
              <Icon name="chevron-down" :size="12" />
            </summary>
            <div class="gl-menu-list">
              <button type="button" @click="copySourceBranch">{{ t("copySourceBranch") }}</button>
              <button type="button" @click="copyLink">{{ t("copyMrUrl") }}</button>
            </div>
          </details>

          <details ref="moreMenu" class="gl-menu">
            <summary class="gl-icon-btn" :aria-label="t('actions')" :title="t('actions')">
              <Icon name="more" :size="16" />
            </summary>
            <div class="gl-menu-list">
              <button
                v-if="mr.state === 'opened'"
                type="button"
                :disabled="readOnly"
                @click="openMenuAction('approve')"
              >
                {{ t("approve") }}
              </button>
              <button
                v-if="mr.state === 'opened'"
                type="button"
                :disabled="readOnly"
                @click="openMenuAction('revoke')"
              >
                {{ t("revoke") }}
              </button>
              <button
                v-if="mr.state === 'opened'"
                type="button"
                :disabled="readOnly"
                @click="openMenuAction('close')"
              >
                {{ t("closeMr") }}
              </button>
              <button
                v-if="mr.state === 'closed'"
                type="button"
                :disabled="readOnly"
                @click="openMenuAction('reopen')"
              >
                {{ t("reopenMr") }}
              </button>
              <button type="button" @click="copyLink">{{ t("copyMrUrl") }}</button>
              <button type="button" @click="closeMenus(); load()">{{ t("refresh") }}</button>
            </div>
          </details>

          <button
            v-if="mr.state === 'opened'"
            class="gl-btn"
            type="button"
            :disabled="readOnly || actionBusy"
            :title="readOnly ? t('readOnlyActionHint') : ''"
            @click="toggleDraft"
          >
            {{ mr.draft ? t("mrMarkReady") : t("mrMarkDraft") }}
          </button>
        </div>
      </header>

      <nav class="gl-detail-tabs" role="tablist">
        <button
          v-for="item in tabs"
          :key="item.id"
          type="button"
          role="tab"
          :class="{ active: tab === item.id }"
          :aria-selected="tab === item.id"
          @click="emit('update:tab', item.id)"
        >
          {{ item.label }}
          <span class="gl-tab-count">{{ item.count }}</span>
        </button>
      </nav>

      <div class="detail-layout" :class="{ 'gl-mr-layout-wide': tab !== 'overview' }">
        <div class="detail-main" :class="{ 'gl-mr-sheet': tab === 'overview' }">
          <template v-if="tab === 'overview'">
            <div v-if="editing" class="card card-body">
              <div class="field">
                <label>{{ t("mrTitle") }}</label>
                <input v-model="editTitle" class="input" type="text" />
              </div>
              <div class="field">
                <label>{{ t("mrDescription") }}</label>
                <textarea v-model="editDescription" class="textarea" rows="6"></textarea>
              </div>
              <div class="composer-actions">
                <button class="btn" type="button" :disabled="saveBusy" @click="editing = false">
                  {{ t("cancel") }}
                </button>
                <button class="btn primary" type="button" :disabled="saveBusy" @click="saveEdit">
                  {{ saveBusy ? t("loading") : t("save") }}
                </button>
              </div>
            </div>

            <div v-else class="gl-mr-desc">
              <MarkdownBlock v-if="mr.description" :source="mr.description" />
              <p v-else class="muted">{{ t("noDescription") }}</p>
            </div>

            <div class="gl-reactions">
              <button
                v-for="state in reactions"
                :key="state.name"
                class="gl-reaction"
                type="button"
                :class="{ active: state.mine }"
                :disabled="readOnly || reactionBusy === state.name"
                :title="state.name"
                @click="toggleReaction(state)"
              >
                <span class="gl-reaction-glyph">{{ state.glyph }}</span>
                <span>{{ state.count }}</span>
              </button>
            </div>

            <section v-if="approvals" class="gl-approvals">
              <button
                class="gl-ready-head"
                type="button"
                :aria-expanded="approvalsOpen"
                @click="approvalsOpen = !approvalsOpen"
              >
                <span
                  class="gl-ready-dot"
                  :data-tone="approvers.length ? 'success' : 'neutral'"
                ></span>
                <strong>{{ t("approvalsCount", { count: approvers.length }) }}</strong>
                <span class="muted small">{{ approvalHint }}</span>
                <span class="grow"></span>
                <Icon name="chevron-down" :size="16" :class="{ 'gl-flip': !approvalsOpen }" />
              </button>

              <div v-if="approvalsOpen" class="gl-ready-body">
                <p v-if="!approvers.length" class="muted small">{{ t("noApprovalsYet") }}</p>
                <span v-else class="gl-people">
                  <span v-for="user in approvers" :key="user.id" class="gl-person">
                    <span class="avatar gl-avatar-xs">{{ initialOf(user) }}</span>
                    {{ user.name || user.username }}
                  </span>
                </span>
                <button
                  v-if="!readOnly && mr.state === 'opened'"
                  class="btn small"
                  type="button"
                  :disabled="actionBusy"
                  @click="myApproval ? revokeNow() : approveNow()"
                >
                  {{ myApproval ? t("revoke") : t("approve") }}
                </button>
              </div>
            </section>

            <section v-if="mr.state === 'opened'" class="gl-ready">
              <button
                class="gl-ready-head"
                type="button"
                :aria-expanded="readyOpen"
                @click="readyOpen = !readyOpen"
              >
                <span class="gl-ready-dot" :data-tone="canMerge ? 'success' : 'danger'"></span>
                <strong>{{ canMerge ? t("mrReadyToMerge") : t("mrMergeBlocked") }}</strong>
                <span class="grow"></span>
                <Icon name="chevron-down" :size="16" :class="{ 'gl-flip': !readyOpen }" />
              </button>

              <div v-if="readyOpen" class="gl-ready-body">
                <div class="gl-ready-options">
                  <label class="checkbox">
                    <input v-model="mergeRemoveSource" type="checkbox" :disabled="readOnly" />
                    {{ t("removeSourceBranch") }}
                  </label>
                  <label class="checkbox">
                    <input v-model="mergeSquash" type="checkbox" :disabled="readOnly" />
                    {{ t("squash") }}
                  </label>
                </div>
                <p class="gl-ready-hint muted small">
                  {{ mergeBlockedReason || t("mergePanelHint", {
                    source: mr.source_branch,
                    target: mr.target_branch,
                  }) }}
                </p>
                <button
                  class="btn primary gl-ready-merge"
                  type="button"
                  :disabled="!canMerge || actionBusy"
                  @click="mergeNow"
                >
                  {{ actionBusy ? t("loading") : t("merge") }}
                </button>
              </div>
            </section>

            <section class="gl-activity">
              <header class="gl-activity-head">
                <h2>{{ t("timeline") }}</h2>
                <span class="grow"></span>
                <label class="gl-activity-filter">
                  <Icon name="funnel" :size="13" />
                  <select v-model="activityFilter" :aria-label="t('activityFilter')">
                    <option value="all">{{ t("activityAll") }}</option>
                    <option value="comments">{{ t("activityComments") }}</option>
                  </select>
                </label>
              </header>

              <div
                v-if="!readOnly"
                class="gl-comment-box"
                @keydown.ctrl.enter.prevent="submitDraft"
              >
                <MarkdownEditor
                  v-model="commentDraft"
                  compact
                  :placeholder="t('writeComment')"
                >
                  <template #actions>
                    <button
                      class="btn primary small"
                      type="button"
                      :disabled="commentBusy || !commentDraft.trim()"
                      @click="submitDraft"
                    >
                      {{ commentBusy ? t("loading") : t("submitComment") }}
                    </button>
                  </template>
                </MarkdownEditor>
              </div>
              <p v-else class="muted small">{{ t("readOnlyActionHint") }}</p>

              <DiscussionList
                class="gl-activity-list"
                :discussions="discussions"
                :read-only="readOnly"
                :hide-system="activityFilter === 'comments'"
                @reply="replyThread"
                @resolve="resolveThread"
              />
            </section>
          </template>

          <div v-else-if="tab === 'changes'" class="card">
            <div v-if="diffsError" class="error-banner gl-diff-error">
              <span class="grow">
                {{ diffErrorHint }}
                <code class="gl-diff-raw">{{ diffsError }}</code>
              </span>
              <a
                v-if="mr.web_url"
                class="link"
                :href="mr.web_url"
                target="_blank"
                rel="noreferrer"
              >
                {{ t("openInGitLab") }}
              </a>
              <button class="btn ghost" type="button" @click="loadDiffs()">{{ t("retry") }}</button>
            </div>
            <p v-if="diffsLoading && !diffs.length" class="loading">{{ t("loading") }}</p>
            <p v-else-if="!diffs.length && !diffsError" class="empty">{{ t("diffEmpty") }}</p>
            <DiffBrowser
              v-if="diffs.length"
              :files="diffs"
              :project-path="project.path_with_namespace"
              :git-ref="headSha"
              :scope="scope"
              :head-sha="headSha"
              :diff-refs="diffRefs"
              :interactive="!readOnly"
              :base-branch="mr.target_branch"
              :total-changes="changesCount"
              :skipped="skippedDiffFiles"
              :loading="diffsLoading"
              @comment="(position) => (inlinePosition = position)"
              @retry="loadMoreDiffs"
            />
          </div>

          <div v-else-if="tab === 'commits'" class="card card-body">
            <p v-if="commitsLoading" class="loading">{{ t("loading") }}</p>
            <p v-else-if="commitsError" class="muted">{{ commitsError }}</p>
            <p v-else-if="!commits.length" class="empty">{{ t("noCommits") }}</p>
            <table v-else class="commit-table">
              <thead>
                <tr>
                  <th>{{ t("commit") }}</th>
                  <th>{{ t("author") }}</th>
                  <th>{{ t("committedAt") }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in commits"
                  :key="item.id"
                  role="button"
                  tabindex="0"
                  @click="emit('open-commit', item.id)"
                  @keydown.enter="emit('open-commit', item.id)"
                  @keydown.space.prevent="emit('open-commit', item.id)"
                >
                  <td>
                    <div class="commit-id mono">{{ item.short_id }}</div>
                    <div>{{ item.title }}</div>
                  </td>
                  <td>{{ item.author_name }}</td>
                  <td>{{ formatDateTime(item.committed_date, locale) }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-else class="card card-body">
            <p v-if="!pipeline" class="empty">{{ t("noPipeline") }}</p>
            <div v-else class="pipeline-summary">
              <div class="mr-meta">
                <StatusBadge :status="pipeline.status" />
                <span class="mono">{{ pipeline.ref }}</span>
                <button
                  class="link mono muted"
                  type="button"
                  :title="t('commit')"
                  @click="emit('open-commit', pipeline.sha)"
                >
                  {{ pipeline.sha.slice(0, 8) }}
                </button>
              </div>
              <button class="btn small" type="button" @click="emit('open-pipeline', pipeline.id)">
                {{ t("pipeline") }} #{{ pipeline.id }}
              </button>
            </div>
          </div>
        </div>

        <aside v-if="tab === 'overview'" class="detail-side gl-mr-side">
          <section class="gl-side-block">
            <header class="gl-side-block-head">
              <h3>{{ t("assigneesCount", { count: mr.assignees.length }) }}</h3>
              <button
                v-if="!readOnly"
                class="link-btn"
                type="button"
                @click="startField('assignees')"
              >
                {{ t("edit") }}
              </button>
            </header>
            <div v-if="editField === 'assignees'" class="gl-side-edit">
              <GitSelect
                mode="multiple"
                :values="draftAssignees"
                :options="memberOptions"
                :loading="optionsLoading"
                :placeholder="t('mrNewAssignee')"
                @update:values="(value) => (draftAssignees = value)"
              />
              <div class="gl-side-edit-actions">
                <button class="btn primary small" type="button" :disabled="fieldBusy" @click="saveField">
                  {{ t("save") }}
                </button>
                <button class="btn ghost small" type="button" @click="editField = ''">
                  {{ t("cancel") }}
                </button>
              </div>
            </div>
            <div v-else class="gl-side-block-body">
              <span v-if="!mr.assignees.length" class="muted">{{ t("noneValue") }}</span>
              <span v-else class="gl-people">
                <span v-for="user in mr.assignees" :key="user.id" class="gl-person">
                  <span class="avatar gl-avatar-xs">{{ initialOf(user) }}</span>
                  {{ user.name || user.username }}
                </span>
              </span>
            </div>
          </section>

          <section class="gl-side-block">
            <header class="gl-side-block-head">
              <h3>{{ t("reviewersCount", { count: mr.reviewers.length }) }}</h3>
              <button
                v-if="!readOnly"
                class="link-btn"
                type="button"
                @click="startField('reviewers')"
              >
                {{ t("edit") }}
              </button>
            </header>
            <div v-if="editField === 'reviewers'" class="gl-side-edit">
              <GitSelect
                mode="multiple"
                :values="draftReviewers"
                :options="memberOptions"
                :loading="optionsLoading"
                :placeholder="t('reviewers')"
                @update:values="(value) => (draftReviewers = value)"
              />
              <div class="gl-side-edit-actions">
                <button class="btn primary small" type="button" :disabled="fieldBusy" @click="saveField">
                  {{ t("save") }}
                </button>
                <button class="btn ghost small" type="button" @click="editField = ''">
                  {{ t("cancel") }}
                </button>
              </div>
            </div>
            <div v-else class="gl-side-block-body">
              <span v-if="!mr.reviewers.length" class="muted">{{ t("noneValue") }}</span>
              <span v-else class="gl-people">
                <span v-for="user in mr.reviewers" :key="user.id" class="gl-person">
                  <span class="avatar gl-avatar-xs">{{ initialOf(user) }}</span>
                  {{ user.name || user.username }}
                </span>
              </span>
            </div>
          </section>

          <section class="gl-side-block">
            <header class="gl-side-block-head">
              <h3>{{ t("labels") }}</h3>
              <button v-if="!readOnly" class="link-btn" type="button" @click="startField('labels')">
                {{ t("edit") }}
              </button>
            </header>
            <div v-if="editField === 'labels'" class="gl-side-edit">
              <GitSelect
                mode="multiple"
                :values="draftLabels"
                :options="labelOptions"
                :loading="optionsLoading"
                :placeholder="t('labelsHint')"
                @update:values="(value) => (draftLabels = value)"
              />
              <div class="gl-side-edit-actions">
                <button class="btn primary small" type="button" :disabled="fieldBusy" @click="saveField">
                  {{ t("save") }}
                </button>
                <button class="btn ghost small" type="button" @click="editField = ''">
                  {{ t("cancel") }}
                </button>
              </div>
            </div>
            <div v-else class="gl-side-block-body">
              <span v-if="!mr.labels.length" class="muted">{{ t("noneValue") }}</span>
              <span v-else class="label-list">
                <span v-for="label in mr.labels" :key="label" class="badge" data-tone="info">
                  {{ label }}
                </span>
              </span>
            </div>
          </section>

          <section class="gl-side-block">
            <header class="gl-side-block-head">
              <h3>{{ t("milestone") }}</h3>
              <button
                v-if="!readOnly"
                class="link-btn"
                type="button"
                @click="startField('milestone')"
              >
                {{ t("edit") }}
              </button>
            </header>
            <div v-if="editField === 'milestone'" class="gl-side-edit">
              <GitSelect
                :value="draftMilestone"
                :options="milestones"
                :loading="optionsLoading"
                :placeholder="t('mrNewNoMilestone')"
                @update:value="(value) => (draftMilestone = value)"
              />
              <div class="gl-side-edit-actions">
                <button
                  class="btn primary small"
                  type="button"
                  :disabled="fieldBusy"
                  @click="saveField"
                >
                  {{ t("save") }}
                </button>
                <button class="btn ghost small" type="button" @click="editField = ''">
                  {{ t("cancel") }}
                </button>
              </div>
            </div>
            <div v-else class="gl-side-block-body">
              <span v-if="!mr.milestone" class="muted">{{ t("noneValue") }}</span>
              <span v-else>{{ mr.milestone.title }}</span>
            </div>
          </section>

          <section class="gl-side-block">
            <header class="gl-side-block-head">
              <h3>{{ t("participantsCount", { count: participants.length }) }}</h3>
            </header>
            <div class="gl-side-block-body">
              <span class="gl-people">
                <span v-for="user in participants" :key="user.id" class="gl-person">
                  <span class="avatar gl-avatar-xs">{{ initialOf(user) }}</span>
                  {{ user.name || user.username }}
                </span>
              </span>
            </div>
          </section>

          <section class="gl-side-block">
            <header class="gl-side-block-head">
              <h3>{{ t("details") }}</h3>
            </header>
            <div class="gl-side-block-body gl-side-facts">
              <div class="gl-fact">
                <span>{{ t("mrSource") }} → {{ t("mrTarget") }}</span>
                <span class="mono small">{{ mr.source_branch }} → {{ mr.target_branch }}</span>
              </div>
              <div class="gl-fact">
                <span>{{ t("changesCount") }}</span>
                <span class="small">{{ mr.changes_count || diffs.length }}</span>
              </div>
              <div class="gl-fact">
                <span>{{ t("notesCount") }}</span>
                <span class="small">{{ mr.user_notes_count }}</span>
              </div>
              <div class="gl-fact">
                <span>{{ t("pipeline") }}</span>
                <StatusBadge v-if="mr.pipeline" :status="mr.pipeline.status" />
                <span v-else class="small muted">-</span>
              </div>
            </div>
          </section>

          <section class="gl-side-block">
            <header class="gl-side-block-head">
              <h3>{{ t("timeTracking") }}</h3>
            </header>
            <div class="gl-side-block-body gl-side-facts">
              <template
                v-if="
                  mr.time_stats &&
                  (mr.time_stats.human_time_estimate || mr.time_stats.human_total_time_spent)
                "
              >
                <div class="gl-fact">
                  <span>{{ t("timeEstimate") }}</span>
                  <span>{{ mr.time_stats.human_time_estimate || "-" }}</span>
                </div>
                <div class="gl-fact">
                  <span>{{ t("timeSpent") }}</span>
                  <span>{{ mr.time_stats.human_total_time_spent || "-" }}</span>
                </div>
              </template>
              <span v-else class="muted">{{ t("noTimeTracked") }}</span>
            </div>
          </section>
        </aside>
      </div>
    </template>

    <ConfirmAction
      v-if="confirmMeta"
      :title="confirmMeta.title"
      :detail="confirmMeta.detail"
      :confirm-label="confirmMeta.label"
      :busy="actionBusy"
      :danger="confirmMeta.danger"
      @confirm="runConfirm"
      @cancel="confirmKind = ''"
    />

    <div v-if="inlinePosition" class="overlay" @click.self="inlinePosition = null">
      <section class="modal">
        <header class="modal-head">
          <h2>{{ t("inlineCommentHint") }}</h2>
          <span class="muted small mono">
            {{ inlinePosition.new_path }}:{{ inlinePosition.new_line ?? inlinePosition.old_line }}
          </span>
          <button class="btn ghost" type="button" @click="inlinePosition = null">×</button>
        </header>
        <div class="modal-body">
          <CommentComposer :busy="commentBusy" @submit="submitInline" />
        </div>
      </section>
    </div>
  </section>
</template>
