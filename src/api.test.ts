import { describe, expect, it, vi } from "vitest";

interface Call {
  method: string;
  connectionId: string;
}

/** 每个用例重置模块，模拟「宿主桥已注入、连接 id 还未设置」的启动瞬间。 */
async function loadApi(contextId?: string): Promise<{
  api: typeof import("./api");
  calls: Call[];
}> {
  vi.resetModules();
  const calls: Call[] = [];
  (globalThis as unknown as { window: unknown }).window = {
    dbxPlugin: {
      context: contextId ? { connectionId: contextId } : {},
      invoke: (method: string, params: Record<string, unknown>) => {
        calls.push({ method, connectionId: String(params.connectionId ?? "") });
        return Promise.resolve({ ok: true });
      },
    },
  };
  const api = await import("./api");
  return { api, calls };
}

describe("invoke connection gating", () => {
  it("holds requests until the host connection id is set", async () => {
    const { api, calls } = await loadApi();

    const pending = api.invoke("gitlab/projects", {});
    await Promise.resolve();
    // 回归：子组件 onMounted 早于 App 注入 connectionId，曾直接打出 missing connectionId
    expect(calls).toHaveLength(0);

    api.setConnectionId("conn-1");
    await pending;
    expect(calls).toEqual([{ method: "gitlab/projects", connectionId: "conn-1" }]);
  });

  it("uses the context id immediately when the host already provided it", async () => {
    const { api, calls } = await loadApi("host-conn");

    await api.invoke("gitlab/projects", {});
    expect(calls).toEqual([{ method: "gitlab/projects", connectionId: "host-conn" }]);
  });

  it("passes extra params through untouched", async () => {
    const { api } = await loadApi("host-conn");
    const seen: Record<string, unknown>[] = [];
    (globalThis as unknown as { window: { dbxPlugin: { invoke: unknown } } }).window.dbxPlugin.invoke = (
      _method: string,
      params: Record<string, unknown>,
    ) => {
      seen.push(params);
      return Promise.resolve({});
    };

    await api.invoke("gitlab/mrs", { scope: "reviewer", page: 2 });
    expect(seen[0]).toMatchObject({ connectionId: "host-conn", scope: "reviewer", page: 2 });
  });
});
