import { describe, expect, it } from "vitest";
import {
  branchIssue,
  buildCreateMrPayload,
  canCompare,
  commitDraft,
  compareSummary,
  defaultMergeOptions,
  nextAutoValue,
  withDraftPrefix,
  type MrDraftInput,
} from "./mrDraft";
import type { Commit, CompareResult } from "../types";

const baseDraft: MrDraftInput = {
  projectId: "demo/project",
  sourceBranch: " feature/sample-http-migration ",
  targetBranch: "master",
  title: "refactor(sample): 调整示例服务调用",
  description: "  - 新增示例配置\n",
  draft: false,
  milestoneId: 0,
  labels: [],
  assigneeIds: [],
  reviewerIds: [],
  removeSourceBranch: true,
  squash: false,
};

describe("branchIssue", () => {
  it("reports which side is missing", () => {
    expect(branchIssue("", "master")).toBe("sourceMissing");
    expect(branchIssue("   ", "master")).toBe("sourceMissing");
    expect(branchIssue("feature/x", "")).toBe("targetMissing");
    expect(branchIssue("feature/x", "  ")).toBe("targetMissing");
  });

  it("rejects the same branch after trimming", () => {
    expect(branchIssue(" feature/x ", "feature/x")).toBe("sameBranch");
  });

  it("accepts two different branches", () => {
    expect(branchIssue("feature/x", "master")).toBe("");
  });
});

describe("canCompare", () => {
  it("mirrors the branch validation", () => {
    expect(canCompare("feature/x", "master")).toBe(true);
    expect(canCompare("feature/x", "feature/x")).toBe(false);
    expect(canCompare("", "master")).toBe(false);
    expect(canCompare("feature/x", "")).toBe(false);
  });
});

describe("withDraftPrefix", () => {
  it("adds the Draft prefix once", () => {
    expect(withDraftPrefix("feat: 示例功能", true)).toBe("Draft: feat: 示例功能");
    expect(withDraftPrefix("Draft: feat: 示例功能", true)).toBe("Draft: feat: 示例功能");
  });

  it("never turns an empty title into a bare prefix", () => {
    expect(withDraftPrefix("   ", true)).toBe("");
  });

  it("strips the prefix when the draft flag is cleared", () => {
    expect(withDraftPrefix("Draft: feat: 示例功能", false)).toBe("feat: 示例功能");
    expect(withDraftPrefix("feat: 示例功能", false)).toBe("feat: 示例功能");
  });
});

describe("defaultMergeOptions", () => {
  it("matches the GitLab new merge request page", () => {
    expect(defaultMergeOptions()).toEqual({ removeSourceBranch: true, squash: false });
  });
});

describe("buildCreateMrPayload", () => {
  it("trims every text field and drops an empty description", () => {
    const payload = buildCreateMrPayload({ ...baseDraft, description: "   " });
    expect(payload.sourceBranch).toBe("feature/sample-http-migration");
    expect(payload.targetBranch).toBe("master");
    expect(payload.title).toBe(baseDraft.title);
    expect(payload.description).toBeUndefined();
  });

  it("keeps the markdown body as typed", () => {
    expect(buildCreateMrPayload(baseDraft).description).toBe(
      "- 新增示例配置",
    );
  });

  it("applies the draft prefix only when asked", () => {
    expect(buildCreateMrPayload({ ...baseDraft, draft: true }).title).toBe(
      `Draft: ${baseDraft.title}`,
    );
    expect(buildCreateMrPayload(baseDraft).title).toBe(baseDraft.title);
  });

  it("de-duplicates people and labels and drops empty entries", () => {
    const payload = buildCreateMrPayload({
      ...baseDraft,
      labels: [" 后端 ", "后端", "", "RPC"],
      assigneeIds: [7, 7, 0],
      reviewerIds: [9, -1, 9, 11],
    });
    expect(payload.labels).toEqual(["后端", "RPC"]);
    expect(payload.assigneeIds).toEqual([7]);
    expect(payload.reviewerIds).toEqual([9, 11]);
  });

  it("omits empty collections and a zero milestone", () => {
    const payload = buildCreateMrPayload(baseDraft);
    expect(payload.labels).toBeUndefined();
    expect(payload.assigneeIds).toBeUndefined();
    expect(payload.reviewerIds).toBeUndefined();
    expect(payload.milestoneId).toBeUndefined();
  });

  it("passes a chosen milestone through", () => {
    expect(buildCreateMrPayload({ ...baseDraft, milestoneId: 12 }).milestoneId).toBe(12);
  });

  it("carries the merge options from the form", () => {
    const payload = buildCreateMrPayload({ ...baseDraft, removeSourceBranch: false, squash: true });
    expect(payload.removeSourceBranch).toBe(false);
    expect(payload.squash).toBe(true);
  });
});

describe("compareSummary", () => {
  const result: CompareResult = {
    commits: [{ id: "a" } as never, { id: "b" } as never],
    diffs: [{ new_path: "a" }, { new_path: "b" }, { new_path: "c" }] as never,
    compare_timeout: false,
    compare_same_ref: false,
  };

  it("counts commits and changed files", () => {
    expect(compareSummary(result)).toEqual({
      commits: 2,
      changes: 3,
      timedOut: false,
      sameRef: false,
    });
  });

  it("flags a compare timeout instead of pretending the branch is empty", () => {
    expect(compareSummary({ ...result, compare_timeout: true })).toEqual({
      commits: 2,
      changes: 3,
      timedOut: true,
      sameRef: false,
    });
  });

  it("flags the same-ref short circuit", () => {
    expect(compareSummary({ ...result, compare_same_ref: true }).sameRef).toBe(true);
  });

  it("falls back to zeros without a result", () => {
    expect(compareSummary(null)).toEqual({
      commits: 0,
      changes: 0,
      timedOut: false,
      sameRef: false,
    });
  });
});

describe("commitDraft", () => {
  const oldest = {
    id: "older",
    title: "refactor(sample): 调整示例服务调用",
    message:
      "refactor(sample): 调整示例服务调用\n\n- 新增示例配置\n- 移除示例接口",
    committed_date: "2026-10-23T10:00:00Z",
  } as unknown as Commit;
  const newest = {
    id: "newer",
    title: "refactor(sample): 移除示例接口",
    message: "refactor(sample): 移除示例接口",
    committed_date: "2026-10-24T10:00:00Z",
  } as unknown as Commit;

  it("takes the oldest commit of the branch, not the latest one", () => {
    expect(commitDraft([newest, oldest]).title).toBe(
      "refactor(sample): 调整示例服务调用",
    );
  });

  it("fills the description with the commit body", () => {
    expect(commitDraft([oldest]).description).toBe(
      "- 新增示例配置\n- 移除示例接口",
    );
  });

  it("leaves the description empty for a subject-only commit", () => {
    expect(commitDraft([{ ...oldest, message: "refactor: 只有标题" }]).description).toBe("");
  });

  it("returns an empty draft without commits", () => {
    expect(commitDraft(null)).toEqual({ title: "", description: "" });
    expect(commitDraft([])).toEqual({ title: "", description: "" });
  });

  it("orders by committed date instead of array position", () => {
    expect(commitDraft([oldest, newest]).title).toBe(
      "refactor(sample): 调整示例服务调用",
    );
  });

  it("falls back to the last entry when dates are missing", () => {
    const undated = { ...oldest, committed_date: "" };
    const other = { ...newest, committed_date: "" };
    expect(commitDraft([other, undated]).title).toBe(undated.title);
  });
});

describe("nextAutoValue", () => {
  it("fills an empty field", () => {
    expect(nextAutoValue("", "", "refactor: x")).toBe("refactor: x");
  });

  it("refreshes a field that still holds the previous automatic value", () => {
    expect(nextAutoValue("refactor: a", "refactor: a", "refactor: b")).toBe("refactor: b");
  });

  it("never overwrites what the user typed", () => {
    expect(nextAutoValue("我的标题 ", "refactor: a", "refactor: b")).toBe("我的标题 ");
  });

  it("treats whitespace-only input as empty", () => {
    expect(nextAutoValue("   ", "", "refactor: x")).toBe("refactor: x");
  });
});
