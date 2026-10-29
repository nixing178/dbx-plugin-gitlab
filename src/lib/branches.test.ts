import { describe, expect, it } from "vitest";
import {
  branchTreeUrl,
  filterBranches,
  groupBranches,
  isStale,
  loadAllBranches,
  newBranchUrl,
  sortBranches,
  STALE_AFTER_DAYS,
} from "./branches";
import type { Branch, BranchPage, Commit } from "../types";

const NOW = Date.parse("2026-10-28T00:00:00Z");
const DAY = 86_400_000;

function commitOn(name: string, daysAgo: number): Commit {
  return {
    id: `${name}-sha`,
    short_id: `${name.slice(0, 8)}`,
    title: `work on ${name}`,
    committed_date: new Date(NOW - daysAgo * DAY).toISOString(),
  } as Commit;
}

function branch(name: string, daysAgo: number | null, extra: Partial<Branch> = {}): Branch {
  return {
    name,
    default: false,
    protected: false,
    commit: daysAgo === null ? null : commitOn(name, daysAgo),
    ...extra,
  };
}

describe("isStale", () => {
  it("uses the three month window from GitLab", () => {
    expect(STALE_AFTER_DAYS).toBe(90);
    expect(isStale(branch("fresh", 89), NOW)).toBe(false);
    expect(isStale(branch("old", 91), NOW)).toBe(true);
  });

  it("treats missing or unparsable commit dates as stale", () => {
    expect(isStale(branch("empty", null), NOW)).toBe(true);
    expect(
      isStale({ ...branch("broken", null), commit: { committed_date: "not-a-date" } as Commit }, NOW),
    ).toBe(true);
  });

  it("honours a custom threshold", () => {
    expect(isStale(branch("week", 8), NOW, 7)).toBe(true);
  });
});

describe("sortBranches", () => {
  it("pins the default branch and orders the rest by last commit", () => {
    const items = [
      branch("feature/a", 2),
      branch("master", 40, { default: true, protected: true }),
      branch("feature/b", 1),
      branch("hotfix", 30),
    ];
    expect(sortBranches(items).map((item) => item.name)).toEqual([
      "master",
      "feature/b",
      "feature/a",
      "hotfix",
    ]);
  });

  it("keeps branches without a commit at the end and does not mutate input", () => {
    const items = [branch("ghost", null), branch("recent", 1)];
    const sorted = sortBranches(items);
    expect(sorted.map((item) => item.name)).toEqual(["recent", "ghost"]);
    expect(items.map((item) => item.name)).toEqual(["ghost", "recent"]);
  });
});

describe("groupBranches", () => {
  it("splits active and stale branches with sorted output", () => {
    const groups = groupBranches(
      [branch("old", 200), branch("master", 120, { default: true }), branch("new", 1)],
      NOW,
    );
    expect(groups.active.map((item) => item.name)).toEqual(["new"]);
    expect(groups.stale.map((item) => item.name)).toEqual(["master", "old"]);
  });
});

describe("filterBranches", () => {
  const items = [
    branch("feature/API-1", 2),
    branch("feature/ui", 1),
    branch("master", 0, { default: true }),
    branch("feature/api-docs", 3),
  ];

  it("returns the list untouched for a blank keyword", () => {
    expect(filterBranches(items, "")).toBe(items);
    expect(filterBranches(items, "   ")).toBe(items);
  });

  it("matches names case-insensitively and trims the keyword", () => {
    expect(filterBranches(items, "API").map((item) => item.name)).toEqual([
      "feature/API-1",
      "feature/api-docs",
    ]);
    expect(filterBranches(items, "  ui  ").map((item) => item.name)).toEqual(["feature/ui"]);
  });

  it("keeps the original order and never mutates the input", () => {
    const snapshot = items.map((item) => item.name);
    expect(filterBranches(items, "feature").map((item) => item.name)).toEqual([
      "feature/API-1",
      "feature/ui",
      "feature/api-docs",
    ]);
    expect(items.map((item) => item.name)).toEqual(snapshot);
    expect(filterBranches(items, "missing")).toEqual([]);
  });
});

function pageOf(names: string[], hasMore: boolean): BranchPage {
  return { items: names.map((name) => branch(name, 1)), page: 1, has_more: hasMore };
}

describe("loadAllBranches", () => {
  it("翻页到 has_more=false 为止并按页序拼接", async () => {
    const seen: number[] = [];
    const result = await loadAllBranches(async (page) => {
      seen.push(page);
      if (page === 1) return pageOf(["feature/a", "feature/b"], true);
      if (page === 2) return pageOf(["master"], true);
      return pageOf(["test"], false);
    });

    expect(seen).toEqual([1, 2, 3]);
    expect(result.items.map((item) => item.name)).toEqual([
      "feature/a",
      "feature/b",
      "master",
      "test",
    ]);
    expect(result.truncated).toBe(false);
    expect(result.nextPage).toBe(4);
  });

  it("达到批次上限时标记截断并给出续拉页码", async () => {
    const seen: number[] = [];
    const result = await loadAllBranches(
      async (page) => {
        seen.push(page);
        return pageOf([`feature/${page}`], true);
      },
      { maxPages: 2 },
    );

    expect(seen).toEqual([1, 2]);
    expect(result.items.map((item) => item.name)).toEqual(["feature/1", "feature/2"]);
    expect(result.truncated).toBe(true);
    expect(result.nextPage).toBe(3);
  });

  it("从 startPage 续拉，供「加载更多」复用", async () => {
    const seen: number[] = [];
    const result = await loadAllBranches(
      async (page) => {
        seen.push(page);
        return pageOf([`feature/${page}`], false);
      },
      { startPage: 3 },
    );

    expect(seen).toEqual([3]);
    expect(result.items.map((item) => item.name)).toEqual(["feature/3"]);
    expect(result.truncated).toBe(false);
    expect(result.nextPage).toBe(4);
  });

  it("空页仍带 has_more 时立即停，避免无限翻页", async () => {
    const result = await loadAllBranches(async () => pageOf([], true));

    expect(result.items).toEqual([]);
    expect(result.truncated).toBe(true);
    expect(result.nextPage).toBe(2);
  });

  it("分页请求失败时原样抛出，由上层展示错误", async () => {
    await expect(
      loadAllBranches(async () => {
        throw new Error("boom");
      }),
    ).rejects.toThrow("boom");
  });
});

describe("branch links", () => {
  it("builds the tree url and escapes the branch name", () => {
    expect(branchTreeUrl("https://gitlab.example.com/demo/app", "feature/a#1")).toBe(
      "https://gitlab.example.com/demo/app/-/tree/feature%2Fa%231",
    );
    expect(branchTreeUrl("https://gitlab.example.com/demo/app/", "master")).toBe(
      "https://gitlab.example.com/demo/app/-/tree/master",
    );
  });

  it("builds the new branch url", () => {
    expect(newBranchUrl("https://gitlab.example.com/demo/app/")).toBe(
      "https://gitlab.example.com/demo/app/-/branches/new",
    );
  });
});
