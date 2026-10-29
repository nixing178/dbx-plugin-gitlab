/**
 * 合并请求列表的筛选契约（纯函数）。
 *
 * 页签顺序、计数归一化、RPC 参数拼装、空状态文案都放在这里，
 * 组件只负责渲染，这样 GitLab 那套「页签 + 筛选行 + 空状态」的规则可被单测覆盖。
 */
import type { MrScope } from "./nav";
import type { MrCounts } from "../types";

export type { MrCounts };

/** GitLab 合并请求页签的状态，顺序即渲染顺序。 */
export type MrState = "opened" | "merged" | "closed" | "all";

/** GitLab 排序下拉里我们支持的字段。 */
export type MrOrderBy = "updated_at" | "created_at";

export type MrSortDir = "asc" | "desc";

export interface MrFilterInput {
  /** 项目路径；空串表示全局视角。 */
  project: string;
  scope: MrScope;
  state: string;
  search: string;
  orderBy?: string;
  sort?: string;
}

/** `gitlab/mrs` 的请求参数。 */
export interface MrListQuery {
  projectId: string;
  scope: string;
  state: string;
  search: string;
  orderBy: MrOrderBy;
  sort: MrSortDir;
}

/** `gitlab/mrCounts` 的请求参数：与列表同源，但不带分页、状态与排序。 */
export interface MrCountsQuery {
  projectId: string;
  scope: string;
  search: string;
}

export interface MrTab {
  state: MrState;
  /** 小于 0 表示计数未知，UI 只隐藏数字。 */
  count: number;
}

export interface MrEmptyCopy {
  titleKey: string;
  hintKey: string;
}

export const MR_STATES: MrState[] = ["opened", "merged", "closed", "all"];

export const MR_ORDER_BYS: MrOrderBy[] = ["updated_at", "created_at"];

export const DEFAULT_MR_ORDER: MrOrderBy = "updated_at";

export const DEFAULT_MR_SORT: MrSortDir = "desc";

/** 计数未知的哨兵值，与后端 -1 对齐。 */
export const UNKNOWN_COUNT = -1;

function isMrState(value: string): value is MrState {
  return (MR_STATES as string[]).includes(value);
}

function isMrOrderBy(value?: string): value is MrOrderBy {
  return typeof value === "string" && (MR_ORDER_BYS as string[]).includes(value);
}

/** 路由里的 state 可能是脏值，统一回退到「开放中」。 */
export function mrListState(raw: string): MrState {
  return isMrState(raw) ? raw : "opened";
}

/** 排序参数归一化：非法值回退到 GitLab 网页的默认（更新时间、降序）。 */
export function mrSort(
  rawOrderBy?: string,
  rawSort?: string,
): { orderBy: MrOrderBy; sort: MrSortDir } {
  const orderBy = isMrOrderBy(rawOrderBy) ? rawOrderBy : DEFAULT_MR_ORDER;
  const sort: MrSortDir = rawSort === "asc" || rawSort === "desc" ? rawSort : DEFAULT_MR_SORT;
  return { orderBy, sort };
}

/** 列表请求参数：项目态走 projectId，全局态才带 scope。 */
export function mrQuery(input: MrFilterInput): MrListQuery {
  const { orderBy, sort } = mrSort(input.orderBy, input.sort);
  const project = input.project.trim();
  const state = mrListState(input.state);
  return {
    projectId: project,
    scope: project ? "" : input.scope,
    state: state === "all" ? "" : state,
    search: input.search.trim(),
    orderBy,
    sort,
  };
}

/** 计数请求参数：与列表同源，避免页签数字和列表内容对不上。 */
export function mrCountsQuery(input: MrFilterInput): MrCountsQuery {
  const query = mrQuery(input);
  return { projectId: query.projectId, scope: query.scope, search: query.search };
}

/** 四个页签及计数；拿不到计数时每项都是 UNKNOWN_COUNT。 */
export function mrTabs(counts: MrCounts | null): MrTab[] {
  return MR_STATES.map((state) => ({
    state,
    count: counts?.[state] ?? UNKNOWN_COUNT,
  }));
}

/** 页签上的数字：未知计数返回空串，不伪装成 0。 */
export function mrCountLabel(count: number): string {
  return Number.isFinite(count) && count >= 0 ? String(count) : "";
}

/** 空状态文案：搜索优先，其次按当前页签状态区分。 */
export function mrEmptyCopy(state: string, search: string): MrEmptyCopy {
  if (search.trim()) {
    return { titleKey: "mrEmptySearchTitle", hintKey: "mrEmptySearchHint" };
  }
  switch (mrListState(state)) {
    case "merged":
      return { titleKey: "mrEmptyMergedTitle", hintKey: "mrEmptyMergedHint" };
    case "closed":
      return { titleKey: "mrEmptyClosedTitle", hintKey: "mrEmptyClosedHint" };
    case "all":
      return { titleKey: "mrEmptyAllTitle", hintKey: "mrEmptyAllHint" };
    default:
      return { titleKey: "mrEmptyOpenedTitle", hintKey: "mrEmptyOpenedHint" };
  }
}
