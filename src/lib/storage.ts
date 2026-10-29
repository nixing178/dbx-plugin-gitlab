/**
 * 轻量持久化。UI 跑在 opaque-origin 的 srcdoc iframe 里，
 * `localStorage` 可能直接抛 SecurityError，所以统一走「探测 + 内存兜底」。
 */

export interface KeyValueStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

const memory = new Map<string, string>();

export const memoryStore: KeyValueStore = {
  getItem: (key) => memory.get(key) ?? null,
  setItem: (key, value) => {
    memory.set(key, value);
  },
  removeItem: (key) => {
    memory.delete(key);
  },
};

let cached: KeyValueStore | null = null;

export function resolveStore(): KeyValueStore {
  if (cached) return cached;
  try {
    const local = globalThis.localStorage;
    if (local) {
      const probe = "__dbx_store_probe__";
      local.setItem(probe, "1");
      local.removeItem(probe);
      cached = local;
      return cached;
    }
  } catch {
    // 沙箱禁用存储，退回内存
  }
  cached = memoryStore;
  return cached;
}

export function readJson<T>(key: string, fallback: T, store: KeyValueStore = resolveStore()): T {
  try {
    const raw = store.getItem(key);
    if (!raw) return fallback;
    const parsed = JSON.parse(raw) as T | null;
    return parsed ?? fallback;
  } catch {
    return fallback;
  }
}

export function writeJson(key: string, value: unknown, store: KeyValueStore = resolveStore()): void {
  try {
    store.setItem(key, JSON.stringify(value));
  } catch {
    // 配额或权限问题都不该影响主流程
  }
}
