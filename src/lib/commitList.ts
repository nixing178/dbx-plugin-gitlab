/**
 * 提交列表的展示契约（纯函数）。
 *
 * 只做「日期分组 / 作者聚合 / 本地筛选 / 头像配色」这类可单测的推导，
 * 组件里不再散落日期格式化与筛选逻辑。
 */
import type { Commit } from "../types";

/** 按本地日期聚合的一组提交；时间不可解析时 key / label 为空串。 */
export interface CommitDayGroup {
  /** 本地日期 `YYYY-MM-DD`。 */
  key: string;
  /** 分组标题：zh `10月 28, 2026` / en `Oct 28, 2026`。 */
  label: string;
  commits: Commit[];
}

/** 作者筛选下拉的候选项，按提交数降序、同数按名称升序。 */
export interface CommitAuthorCount {
  name: string;
  count: number;
}

export interface CommitFilter {
  /** 作者名精确匹配；空串表示不筛选。 */
  author?: string;
  /** 标题或正文关键字，忽略大小写与首尾空白；空串表示不筛选。 */
  query?: string;
}

const MONTHS_EN = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

interface DayParts {
  year: number;
  /** 1-12，分组标题不补零。 */
  month: number;
  day: number;
}

function isChinese(locale: string): boolean {
  return String(locale ?? "")
    .toLowerCase()
    .startsWith("zh");
}

/** 取本地年月日：分组按用户时区，不能用 UTC。 */
function dayParts(iso: string): DayParts | null {
  if (!iso) return null;
  const time = Date.parse(iso);
  if (Number.isNaN(time)) return null;
  const date = new Date(time);
  if (Number.isNaN(date.getTime())) return null;
  return { year: date.getFullYear(), month: date.getMonth() + 1, day: date.getDate() };
}

function pad2(value: number): string {
  return String(value).padStart(2, "0");
}

function dayLabel(parts: DayParts, locale: string): string {
  if (isChinese(locale)) return `${parts.month}月 ${parts.day}, ${parts.year}`;
  return `${MONTHS_EN[parts.month - 1]} ${parts.day}, ${parts.year}`;
}

function commitIso(commit: Commit): string {
  return commit.committed_date || commit.authored_date || "";
}

/** 日期分组标题；空提交时间返回空串，交给 UI 决定是否省略表头。 */
export function formatCommitDay(iso: string, locale: string): string {
  const parts = dayParts(iso);
  return parts ? dayLabel(parts, locale) : "";
}

/** 按 `committed_date`（缺失时回退 `authored_date`）分组，保持入参顺序。 */
export function groupCommitsByDay(commits: Commit[], locale: string): CommitDayGroup[] {
  const groups: CommitDayGroup[] = [];

  for (const commit of commits) {
    const parts = dayParts(commitIso(commit));
    const key = parts ? `${parts.year}-${pad2(parts.month)}-${pad2(parts.day)}` : "";
    const last = groups[groups.length - 1];
    if (last && last.key === key) {
      last.commits.push(commit);
      continue;
    }
    groups.push({ key, label: parts ? dayLabel(parts, locale) : "", commits: [commit] });
  }

  return groups;
}

/** 作者下拉候选项；姓名为空白的提交不计入。 */
export function commitAuthorOptions(commits: Commit[]): CommitAuthorCount[] {
  const counts = new Map<string, number>();

  for (const commit of commits) {
    const name = (commit.author_name ?? "").trim();
    if (!name) continue;
    counts.set(name, (counts.get(name) ?? 0) + 1);
  }

  return [...counts.entries()]
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => {
      if (a.count !== b.count) return b.count - a.count;
      if (a.name === b.name) return 0;
      return a.name < b.name ? -1 : 1;
    });
}

/** 作者 + 关键字筛选；返回新数组，不修改入参。 */
export function filterCommits(commits: Commit[], filter: CommitFilter): Commit[] {
  const author = (filter.author ?? "").trim();
  const query = (filter.query ?? "").trim().toLowerCase();

  if (!author && !query) return commits.slice();

  return commits.filter((commit) => {
    if (author && (commit.author_name ?? "").trim() !== author) return false;
    if (!query) return true;
    const title = (commit.title ?? "").toLowerCase();
    const message = (commit.message ?? "").toLowerCase();
    return title.includes(query) || message.includes(query);
  });
}

/** 头像配色档位 0-5，按作者名稳定取模；空名固定为 0。 */
export function commitAvatarTone(name: string): number {
  const text = (name ?? "").trim();
  if (!text) return 0;

  let hash = 0;
  for (let index = 0; index < text.length; index += 1) {
    hash = (hash * 31 + text.charCodeAt(index)) % 1_000_003;
  }
  return hash % 6;
}
