/**
 * 新建合并请求页的纯逻辑。
 *
 * 页面是「先选分支 → 比较 → 再填表单」的两段式，这里只放可单测的规则：
 * 分支校验、Draft 前缀、创建参数组装、比较结果摘要。
 */
import type { Commit, CompareResult } from "../types";

/** 表单的完整状态；比较结果不进来，避免纯函数依赖网络对象。 */
export interface MrDraftInput {
  projectId: string;
  sourceBranch: string;
  targetBranch: string;
  title: string;
  description: string;
  draft: boolean;
  milestoneId: number;
  labels: string[];
  assigneeIds: number[];
  reviewerIds: number[];
  removeSourceBranch: boolean;
  squash: boolean;
}

/** 创建接口的入参；空集合与 0 值不发送，交给后端默认值。 */
export interface CreateMrPayload {
  projectId: string;
  sourceBranch: string;
  targetBranch: string;
  title: string;
  description?: string;
  squash: boolean;
  removeSourceBranch: boolean;
  labels?: string[];
  assigneeIds?: number[];
  reviewerIds?: number[];
  milestoneId?: number;
}

export type BranchIssue = "" | "sourceMissing" | "targetMissing" | "sameBranch";

const DRAFT_PREFIX = /^draft:\s*/i;

export function branchIssue(source: string, target: string): BranchIssue {
  const from = source.trim();
  const to = target.trim();
  if (!from) return "sourceMissing";
  if (!to) return "targetMissing";
  if (from === to) return "sameBranch";
  return "";
}

export function canCompare(source: string, target: string): boolean {
  return branchIssue(source, target) === "";
}

/** GitLab 用标题前缀表示草稿；来回切换不能叠加也不能漏删。 */
export function withDraftPrefix(title: string, draft: boolean): string {
  const trimmed = title.trim().replace(DRAFT_PREFIX, "");
  if (!trimmed) return "";
  return draft ? `Draft: ${trimmed}` : trimmed;
}

export function defaultMergeOptions(): { removeSourceBranch: boolean; squash: boolean } {
  return { removeSourceBranch: true, squash: false };
}

function uniqueLabels(labels: string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const raw of labels) {
    const label = raw.trim();
    if (!label || seen.has(label)) continue;
    seen.add(label);
    result.push(label);
  }
  return result;
}

function uniqueIDs(ids: number[]): number[] {
  const seen = new Set<number>();
  const result: number[] = [];
  for (const id of ids) {
    if (!Number.isFinite(id) || id <= 0 || seen.has(id)) continue;
    seen.add(id);
    result.push(id);
  }
  return result;
}

export function buildCreateMrPayload(input: MrDraftInput): CreateMrPayload {
  const description = input.description.trim();
  const labels = uniqueLabels(input.labels);
  const assigneeIds = uniqueIDs(input.assigneeIds);
  const reviewerIds = uniqueIDs(input.reviewerIds);

  const payload: CreateMrPayload = {
    projectId: input.projectId,
    sourceBranch: input.sourceBranch.trim(),
    targetBranch: input.targetBranch.trim(),
    title: withDraftPrefix(input.title, input.draft),
    squash: input.squash,
    removeSourceBranch: input.removeSourceBranch,
  };
  if (description) payload.description = description;
  if (labels.length) payload.labels = labels;
  if (assigneeIds.length) payload.assigneeIds = assigneeIds;
  if (reviewerIds.length) payload.reviewerIds = reviewerIds;
  if (input.milestoneId > 0) payload.milestoneId = input.milestoneId;
  return payload;
}

export interface CompareSummary {
  commits: number;
  changes: number;
  timedOut: boolean;
  sameRef: boolean;
}

/** timeout / same_ref 必须显式暴露：否则会显示成「0 提交 0 变更」，像是分支没差异。 */
export function compareSummary(result: CompareResult | null): CompareSummary {
  if (!result) {
    return { commits: 0, changes: 0, timedOut: false, sameRef: false };
  }
  return {
    commits: result.commits?.length ?? 0,
    changes: result.diffs?.length ?? 0,
    timedOut: Boolean(result.compare_timeout),
    sameRef: Boolean(result.compare_same_ref),
  };
}

export interface CommitDraft {
  title: string;
  description: string;
}

/**
 * 用分支的第一个提交预填标题与描述。
 *
 * 「第一个」按提交时间取最早的一条（GitLab compare 是倒序返回，
 * 数组末尾通常是分支拉出后第一个提交，但日期缺失或乱序时不能只靠位置）。
 * 标题取 subject，描述取 message 去掉 subject 的正文；只有标题的提交描述留空。
 */
export function commitDraft(commits: Commit[] | null | undefined): CommitDraft {
  const list = commits ?? [];
  if (!list.length) return { title: "", description: "" };

  const oldest = pickOldestCommit(list);
  const message = (oldest.message ?? "").replace(/\r\n/g, "\n");
  const subject = (oldest.title || message.split("\n")[0] || "").trim();
  return { title: subject, description: stripCommitBody(message) };
}

function pickOldestCommit(commits: Commit[]): Commit {
  let oldest = commits[commits.length - 1];
  let oldestAt = Date.parse(oldest.committed_date ?? "");
  for (const commit of commits) {
    const at = Date.parse(commit.committed_date ?? "");
    if (Number.isNaN(at)) continue;
    if (Number.isNaN(oldestAt) || at < oldestAt) {
      oldest = commit;
      oldestAt = at;
    }
  }
  return oldest;
}

function stripCommitBody(message: string): string {
  const trimmed = message.trim();
  if (!trimmed) return "";
  const index = trimmed.indexOf("\n");
  if (index < 0) return "";
  return trimmed.slice(index + 1).trim();
}

/**
 * 自动填充只在「字段为空」或「内容仍是上次自动值」时生效。
 * 用户手动改过的内容不能被重新比较覆盖。
 */
export function nextAutoValue(current: string, previousAuto: string, next: string): string {
  if (!current.trim()) return next;
  if (previousAuto && current === previousAuto) return next;
  return current;
}
