/**
 * 议题元信息展示契约（纯函数）。
 */

/** 不同 GitLab 版本的 `task_completion_status` 可能缺失或字段不完整。 */
export interface TaskCompletionLike {
  count?: number;
  completed_count?: number;
}

/** 任务完成度文案，例如 `1/2`；没有任务或字段缺失时返回空串。 */
export function taskProgress(value?: TaskCompletionLike | null): string {
  const count = Number(value?.count ?? 0);
  if (!Number.isFinite(count) || count <= 0) return "";

  const rawDone = Number(value?.completed_count ?? 0);
  const done = Number.isFinite(rawDone) ? Math.min(Math.max(rawDone, 0), count) : 0;
  return `${done}/${count}`;
}
