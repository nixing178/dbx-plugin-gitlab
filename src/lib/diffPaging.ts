/**
 * 「变更」页的分页策略。
 *
 * 大 MR 的 diff 单页就可能几十 MB，一次拉完经常撞上限。GitLab 的做法是
 * 先加载一部分文件、其余折叠；这里把阈值抽出来，方便单测与统一口径。
 */
/** 一页 5 个文件：单页响应体积小，某一页失败也不会把整页拖垮。 */
export const DIFF_PAGE_SIZE = 5;

/** 首屏最多加载的文件数；超出部分等用户点「展开所有文件」。 */
export const DIFF_INITIAL_FILES = 20;

export function diffInitialPages(totalChanges: number): number {
  const target = Math.min(Math.max(totalChanges, DIFF_PAGE_SIZE), DIFF_INITIAL_FILES);
  return Math.max(Math.ceil(target / DIFF_PAGE_SIZE), 1);
}

/** totalChanges 为 0 表示实例没给变更数，此时不提示缺失，避免把未知当缺失。 */
export function diffMissingFiles(totalChanges: number, loaded: number): number {
  if (totalChanges <= 0) return 0;
  return Math.max(totalChanges - loaded, 0);
}
