import { describe, expect, it } from "vitest";
import { memoryStore } from "./storage";
import { loadViewed, saveViewed, toggleKey, viewedKey, viewedScope } from "./viewed";

describe("viewedScope", () => {
  it("scopes the viewed set to one merge request", () => {
    expect(viewedScope("demo/app", 12)).toBe("demo/app!12");
    expect(viewedScope("demo/app", 12)).not.toBe(viewedScope("demo/app", 13));
  });
});

describe("viewedKey", () => {
  it("invalidates marks when the head commit changes", () => {
    expect(viewedKey("abc123", "src/main.kt")).toBe("abc123:src/main.kt");
    expect(viewedKey("abc123", "src/main.kt")).not.toBe(viewedKey("def456", "src/main.kt"));
  });
});

describe("toggleKey", () => {
  it("adds and removes without mutating the input", () => {
    const original = ["a"];
    const added = toggleKey(original, "b");
    expect(added).toEqual(["a", "b"]);
    expect(original).toEqual(["a"]);
    expect(toggleKey(added, "b")).toEqual(["a"]);
  });
});

describe("loadViewed", () => {
  it("returns an empty list when nothing was stored", () => {
    expect(loadViewed("missing/scope!1", memoryStore)).toEqual([]);
  });

  it("survives corrupted JSON", () => {
    memoryStore.setItem("dbx.gitlab.viewed.broken!1", "{not json");
    expect(loadViewed("broken!1", memoryStore)).toEqual([]);
  });

  it("ignores non string entries", () => {
    memoryStore.setItem("dbx.gitlab.viewed.mixed!1", JSON.stringify(["a", 3, null, "b"]));
    expect(loadViewed("mixed!1", memoryStore)).toEqual(["a", "b"]);
  });

  it("round-trips through saveViewed", () => {
    saveViewed("round!1", ["k1", "k2"], memoryStore);
    expect(loadViewed("round!1", memoryStore)).toEqual(["k1", "k2"]);
  });

  it("treats a non array payload as empty", () => {
    memoryStore.setItem("dbx.gitlab.viewed.object!1", JSON.stringify({ a: 1 }));
    expect(loadViewed("object!1", memoryStore)).toEqual([]);
  });
});
