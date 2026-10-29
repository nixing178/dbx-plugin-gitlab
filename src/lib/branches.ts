import type { Branch, BranchPage, Commit } from "../types";

/** GitLab 网页把三个月没有提交的分支归到「非活跃」，这里保持同一口径。 */
export const STALE_AFTER_DAYS = 90;

/** 一次最多自动翻多少页分支（100 条/页）；超过就留给「加载更多」续拉。 */
export const BRANCH_PAGE_BATCH = 10;

const DAY_MS = 86_400_000;

export interface BranchGroups {
  active: Branch[];
  stale: Branch[];
}

function commitTime(commit: Commit | null | undefined): number {
  const iso = commit?.committed_date;
  if (!iso) return Number.NaN;
  return Date.parse(iso);
}

/** 没有提交（空仓库/权限不足）也按非活跃处理，避免概览里出现无日期的行。 */
export function isStale(
  branch: Branch,
  now = Date.now(),
  staleDays = STALE_AFTER_DAYS,
): boolean {
  const time = commitTime(branch.commit);
  if (Number.isNaN(time)) return true;
  return now - time > staleDays * DAY_MS;
}

/** 默认分支固定置顶（GitLab 的默认分支不参与按时间排序），其余按最后提交倒序。 */
export function sortBranches(items: Branch[]): Branch[] {
  return [...items].sort((left, right) => {
    if (left.default !== right.default) return left.default ? -1 : 1;
    const leftTime = commitTime(left.commit);
    const rightTime = commitTime(right.commit);
    const leftKnown = !Number.isNaN(leftTime);
    const rightKnown = !Number.isNaN(rightTime);
    if (leftKnown && rightKnown && leftTime !== rightTime) return rightTime - leftTime;
    if (leftKnown !== rightKnown) return leftKnown ? -1 : 1;
    return left.name.localeCompare(right.name);
  });
}

export function groupBranches(
  items: Branch[],
  now = Date.now(),
  staleDays = STALE_AFTER_DAYS,
): BranchGroups {
  const active: Branch[] = [];
  const stale: Branch[] = [];
  for (const branch of sortBranches(items)) {
    if (isStale(branch, now, staleDays)) stale.push(branch);
    else active.push(branch);
  }
  return { active, stale };
}

/** 客户端即时筛选：空关键字原样返回，避免每次输入都重建数组。 */
export function filterBranches(items: Branch[], keyword: string): Branch[] {
  const needle = keyword.trim().toLowerCase();
  if (!needle) return items;
  return items.filter((item) => item.name.toLowerCase().includes(needle));
}

export interface BranchBatch {
  items: Branch[];
  /** true 表示还有后续页没拉，界面要保留「加载更多」。 */
  truncated: boolean;
  /** 下一批续拉应从哪一页开始。 */
  nextPage: number;
}

/**
 * 连续翻页直到 has_more=false。
 * GitLab REST 分支列表固定按分支名升序且不支持 sort 参数，只取一页会漏掉
 * master/test 这类字母序靠后的分支，所以按批次拉全量，排序交给 groupBranches。
 * 空页却声称还有下一页时立即停，避免实例分页异常时无限翻页。
 */
export async function loadAllBranches(
  fetchPage: (page: number) => Promise<BranchPage>,
  options: { startPage?: number; maxPages?: number } = {},
): Promise<BranchBatch> {
  const startPage = Math.max(1, Math.floor(options.startPage ?? 1) || 1);
  const maxPages = Math.max(1, Math.floor(options.maxPages ?? BRANCH_PAGE_BATCH) || 1);
  const items: Branch[] = [];
  let page = startPage;

  for (let index = 0; index < maxPages; index += 1) {
    const result = await fetchPage(page);
    page += 1;
    items.push(...result.items);
    if (!result.has_more) return { items, truncated: false, nextPage: page };
    if (!result.items.length) break;
  }

  return { items, truncated: true, nextPage: page };
}

function trimBase(webUrl: string): string {
  return (webUrl ?? "").replace(/\/+$/, "");
}

/** 分支文件树页；分支名里的 `/`、`#` 必须编码，否则会被当成路径和锚点。 */
export function branchTreeUrl(webUrl: string, name: string): string {
  return `${trimBase(webUrl)}/-/tree/${encodeURIComponent(name)}`;
}

export function newBranchUrl(webUrl: string): string {
  return `${trimBase(webUrl)}/-/branches/new`;
}
