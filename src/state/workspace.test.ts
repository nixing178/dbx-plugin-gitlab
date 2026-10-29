import { afterEach, describe, expect, it, vi } from "vitest";
import type { KeyValueStore } from "../lib/storage";

const STORAGE_KEY = "dbx.gitlab.workspace.v1";

function createStore(): KeyValueStore {
  const values = new Map<string, string>();
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => {
      values.set(key, value);
    },
    removeItem: (key) => {
      values.delete(key);
    },
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.resetModules();
});

describe("workspace pin persistence", () => {
  it("writes a pin before an immediate reload can discard the pending watcher", async () => {
    const store = createStore();
    vi.stubGlobal("localStorage", store);
    vi.resetModules();
    const workspace = await import("./workspace");

    workspace.togglePinnedProjectNav("mrs");

    const persisted = JSON.parse(store.getItem(STORAGE_KEY) ?? "null") as { pinned?: string[] } | null;
    expect(persisted?.pinned).toEqual(["mrs"]);
  });

  it("writes pins to DBX host storage when localStorage is unavailable", async () => {
    const set = vi.fn(async (_key: string, _value: unknown) => {});
    vi.stubGlobal("localStorage", {
      setItem: () => {
        throw new Error("SecurityError");
      },
    });
    vi.stubGlobal("window", {
      dbxPlugin: { storage: { get: vi.fn(async () => null), set } },
    });
    vi.resetModules();
    const workspace = await import("./workspace");

    workspace.togglePinnedProjectNav("mrs");

    expect(set).toHaveBeenCalledWith(STORAGE_KEY, expect.objectContaining({ pinned: ["mrs"] }));
  });

  it("restores pins from DBX host storage during startup hydration", async () => {
    vi.stubGlobal("localStorage", {
      setItem: () => {
        throw new Error("SecurityError");
      },
    });
    vi.stubGlobal("window", {
      dbxPlugin: {
        storage: {
          get: vi.fn(async () => ({ pinned: ["ci", "mrs"] })),
          set: vi.fn(async () => {}),
        },
      },
    });
    vi.resetModules();
    const workspace = await import("./workspace");
    const hydrate = (workspace as { hydrateWorkspace?: () => Promise<void> }).hydrateWorkspace;

    expect(hydrate).toBeTypeOf("function");
    await hydrate?.();

    expect(workspace.pinnedProjectNav.value).toEqual(["ci", "mrs"]);
  });
});
