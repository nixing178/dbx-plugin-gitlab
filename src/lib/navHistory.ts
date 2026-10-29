/**
 * 页面导航历史（纯数据，无副作用）。
 *
 * 插件 UI 跑在 opaque-origin 的 iframe 里，浏览器 History API 不可用
 * （`pushState` 会抛 SecurityError，侧键默认导航也不属于插件），
 * 所以历史自己维护：`entries` 是访问过的页面序列，`index` 指向当前页。
 */

export interface NavHistory<T> {
  entries: T[];
  index: number;
}

export const DEFAULT_HISTORY_LIMIT = 30;

export function createHistory<T>(initial: T): NavHistory<T> {
  return { entries: [initial], index: 0 };
}

export function currentOf<T>(history: NavHistory<T>): T {
  return history.entries[history.index];
}

/** 压入新页：前进分支作废，超出上限丢最旧的一条。 */
export function pushEntry<T>(
  history: NavHistory<T>,
  entry: T,
  limit: number = DEFAULT_HISTORY_LIMIT,
): NavHistory<T> {
  const cap = Number.isFinite(limit) ? Math.max(1, Math.floor(limit)) : DEFAULT_HISTORY_LIMIT;
  const kept = history.entries.slice(0, history.index + 1);
  kept.push(entry);
  const entries = kept.length > cap ? kept.slice(kept.length - cap) : kept;
  return { entries, index: entries.length - 1 };
}

/** 同页参数变化：原地替换，不动历史长度与指针。 */
export function replaceEntry<T>(history: NavHistory<T>, entry: T): NavHistory<T> {
  const entries = history.entries.slice();
  entries[history.index] = entry;
  return { entries, index: history.index };
}

/** -1 后退 / +1 前进；越界返回 null，调用方保持原状。 */
export function step<T>(history: NavHistory<T>, delta: number): NavHistory<T> | null {
  const index = history.index + delta;
  if (index < 0 || index >= history.entries.length) return null;
  return { entries: history.entries, index };
}

export function canGoBack<T>(history: NavHistory<T>): boolean {
  return history.index > 0;
}

export function canGoForward<T>(history: NavHistory<T>): boolean {
  return history.index < history.entries.length - 1;
}
