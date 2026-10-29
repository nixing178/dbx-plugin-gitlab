import { describe, expect, it } from "vitest";
import type { Commit } from "../types";
import {
  commitAuthorOptions,
  commitAvatarTone,
  filterCommits,
  formatCommitDay,
  groupCommitsByDay,
} from "./commitList";

/** 用本地时间造 ISO：分组按本地日，写死 UTC 会随运行机器时区漂移。 */
function localIso(year: number, month: number, day: number, hour = 12): string {
  return new Date(year, month - 1, day, hour, 0, 0).toISOString();
}

function localKey(year: number, month: number, day: number): string {
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${year}-${pad(month)}-${pad(day)}`;
}

function makeCommit(patch: Partial<Commit> = {}): Commit {
  return {
    id: "abc123",
    short_id: "abc123",
    title: "fix: 修复",
    message: "fix: 修复",
    author_name: "测试作者",
    author_email: "author@example.com",
    authored_date: localIso(2026, 10, 28, 9),
    committer_name: "测试作者",
    committed_date: localIso(2026, 10, 28, 10),
    web_url: "https://gitlab.example.com/demo/app/-/commit/abc123",
    parent_ids: [],
    ...patch,
  };
}

describe("formatCommitDay", () => {
  it("formats Chinese and English day headers", () => {
    expect(formatCommitDay(localIso(2026, 10, 28), "zh")).toBe("10月 28, 2026");
    expect(formatCommitDay(localIso(2026, 10, 28), "zh-CN")).toBe("10月 28, 2026");
    expect(formatCommitDay(localIso(2026, 10, 28), "en")).toBe("Oct 28, 2026");
  });

  it("does not pad month and day", () => {
    expect(formatCommitDay(localIso(2026, 1, 5), "zh")).toBe("1月 5, 2026");
    expect(formatCommitDay(localIso(2026, 12, 31), "en")).toBe("Dec 31, 2026");
  });

  it("returns an empty string for invalid input", () => {
    expect(formatCommitDay("", "zh")).toBe("");
    expect(formatCommitDay("not-a-date", "en")).toBe("");
  });
});

describe("groupCommitsByDay", () => {
  it("groups consecutive commits of the same local day and keeps order", () => {
    const commits = [
      makeCommit({ id: "c1", committed_date: localIso(2026, 10, 28, 9) }),
      makeCommit({ id: "c2", committed_date: localIso(2026, 10, 28, 18) }),
      makeCommit({ id: "c3", committed_date: localIso(2026, 10, 23, 8) }),
      makeCommit({ id: "c4", committed_date: localIso(2026, 10, 23, 20) }),
      makeCommit({ id: "c5", committed_date: localIso(2026, 10, 22, 11) }),
    ];

    const groups = groupCommitsByDay(commits, "zh");
    expect(groups.map((group) => [group.key, group.label, group.commits.length])).toEqual([
      [localKey(2026, 10, 28), "10月 28, 2026", 2],
      [localKey(2026, 10, 23), "10月 23, 2026", 2],
      [localKey(2026, 10, 22), "10月 22, 2026", 1],
    ]);
    expect(groups[0].commits.map((commit) => commit.id)).toEqual(["c1", "c2"]);
  });

  it("falls back to the authored date when the committed date is missing", () => {
    const groups = groupCommitsByDay(
      [makeCommit({ committed_date: "", authored_date: localIso(2026, 10, 20, 9) })],
      "en",
    );
    expect(groups[0].key).toBe(localKey(2026, 10, 20));
    expect(groups[0].label).toBe("Oct 20, 2026");
  });

  it("keeps unparsable dates in one placeholder group", () => {
    const groups = groupCommitsByDay(
      [
        makeCommit({ id: "bad1", committed_date: "", authored_date: "" }),
        makeCommit({ id: "bad2", committed_date: "not-a-date", authored_date: "" }),
      ],
      "zh",
    );
    expect(groups).toHaveLength(1);
    expect(groups[0].key).toBe("");
    expect(groups[0].label).toBe("");
    expect(groups[0].commits.map((commit) => commit.id)).toEqual(["bad1", "bad2"]);
  });

  it("returns an empty list for empty input", () => {
    expect(groupCommitsByDay([], "zh")).toEqual([]);
  });
});

describe("commitAuthorOptions", () => {
  it("counts authors and sorts by count then name", () => {
    const commits = [
      makeCommit({ author_name: "user6" }),
      makeCommit({ author_name: "user2" }),
      makeCommit({ author_name: "user2" }),
      makeCommit({ author_name: "Sample" }),
    ];

    expect(commitAuthorOptions(commits)).toEqual([
      { name: "user2", count: 2 },
      { name: "Sample", count: 1 },
      { name: "user6", count: 1 },
    ]);
  });

  it("skips blank author names", () => {
    expect(
      commitAuthorOptions([makeCommit({ author_name: "  " }), makeCommit({ author_name: "测试作者" })]),
    ).toEqual([{ name: "测试作者", count: 1 }]);
  });

  it("returns an empty list for empty input", () => {
    expect(commitAuthorOptions([])).toEqual([]);
  });
});

describe("filterCommits", () => {
  const commits = [
    makeCommit({
      id: "c1",
      title: "feat(sample): 示例信息新增导出功能",
      message: "feat(sample): 示例信息新增导出功能",
      author_name: "user6",
    }),
    makeCommit({
      id: "c2",
      title: "fix(sample): 修复示例详情查询空指针异常",
      message: "fix(sample): 修复示例详情查询空指针异常\n\n复现：导入重复数据",
      author_name: "user3",
    }),
    makeCommit({
      id: "c3",
      title: "Merge branch 'feature-sample' into 'master'",
      message: "Merge branch 'feature-sample' into 'master'",
      author_name: "user2",
    }),
  ];

  it("returns everything when no filter is active", () => {
    expect(filterCommits(commits, {}).map((commit) => commit.id)).toEqual(["c1", "c2", "c3"]);
    expect(
      filterCommits(commits, { author: "", query: "   " }).map((commit) => commit.id),
    ).toEqual(["c1", "c2", "c3"]);
  });

  it("filters by author exactly", () => {
    expect(filterCommits(commits, { author: "user3" }).map((commit) => commit.id)).toEqual([
      "c2",
    ]);
    expect(filterCommits(commits, { author: "ser" })).toEqual([]);
  });

  it("searches titles and messages case-insensitively", () => {
    expect(filterCommits(commits, { query: "示例" }).map((commit) => commit.id)).toEqual([
      "c1",
      "c2",
    ]);
    expect(filterCommits(commits, { query: "MERGE" }).map((commit) => commit.id)).toEqual(["c3"]);
    expect(filterCommits(commits, { query: "重复数据" }).map((commit) => commit.id)).toEqual([
      "c2",
    ]);
    expect(filterCommits(commits, { query: "  示例  " }).map((commit) => commit.id)).toEqual([
      "c1",
      "c2",
    ]);
  });

  it("combines author and query filters", () => {
    expect(
      filterCommits(commits, { author: "user6", query: "示例" }).map((commit) => commit.id),
    ).toEqual(["c1"]);
    expect(filterCommits(commits, { author: "user6", query: "空指针" })).toEqual([]);
  });

  it("does not mutate the input list", () => {
    const result = filterCommits(commits, { author: "user3" });
    expect(result).not.toBe(commits);
    expect(commits).toHaveLength(3);
  });
});

describe("commitAvatarTone", () => {
  const names = ["测试作者", "user2", "user6", "user3", "Sample", "GitLab"];

  it("returns a stable tone within 0-5", () => {
    for (const name of names) {
      const tone = commitAvatarTone(name);
      expect(Number.isInteger(tone)).toBe(true);
      expect(tone).toBeGreaterThanOrEqual(0);
      expect(tone).toBeLessThanOrEqual(5);
      expect(commitAvatarTone(name)).toBe(tone);
    }
  });

  it("spreads typical authors across several tones", () => {
    expect(new Set(names.map(commitAvatarTone)).size).toBeGreaterThanOrEqual(3);
  });

  it("uses tone 0 for blank names", () => {
    expect(commitAvatarTone("")).toBe(0);
    expect(commitAvatarTone("   ")).toBe(0);
  });
});
