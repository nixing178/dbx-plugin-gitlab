import { resolveStore, type KeyValueStore } from "./storage";

/** 每个 MR 一份已读集合，force-push 换了 head_sha 后 key 变化，标记自动失效。 */
const PREFIX = "dbx.gitlab.viewed.";

export function viewedScope(project: string, iid: number): string {
  return `${project}!${iid}`;
}

export function viewedKey(headSha: string, path: string): string {
  return `${headSha}:${path}`;
}

export function loadViewed(scope: string, store: KeyValueStore = resolveStore()): string[] {
  try {
    const raw = store.getItem(PREFIX + scope);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item): item is string => typeof item === "string");
  } catch {
    return [];
  }
}

export function saveViewed(
  scope: string,
  keys: string[],
  store: KeyValueStore = resolveStore(),
): void {
  try {
    store.setItem(PREFIX + scope, JSON.stringify(keys));
  } catch {
    // 存储不可用时只是不记录已读，不影响浏览
  }
}

export function toggleKey(keys: string[], key: string): string[] {
  return keys.includes(key) ? keys.filter((item) => item !== key) : [...keys, key];
}
