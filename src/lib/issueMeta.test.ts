import { describe, expect, it } from "vitest";
import { taskProgress } from "./issueMeta";

describe("taskProgress", () => {
  it("formats completed over total", () => {
    expect(taskProgress({ count: 2, completed_count: 1 })).toBe("1/2");
    expect(taskProgress({ count: 12, completed_count: 12 })).toBe("12/12");
  });

  it("treats a missing completed count as zero", () => {
    expect(taskProgress({ count: 3 })).toBe("0/3");
  });

  it("hides when the instance does not return the field", () => {
    expect(taskProgress(undefined)).toBe("");
    expect(taskProgress(null)).toBe("");
    expect(taskProgress({})).toBe("");
    expect(taskProgress({ count: 0, completed_count: 0 })).toBe("");
  });

  it("clamps out-of-range and invalid numbers", () => {
    expect(taskProgress({ count: 2, completed_count: 5 })).toBe("2/2");
    expect(taskProgress({ count: 2, completed_count: -1 })).toBe("0/2");
    expect(taskProgress({ count: 2, completed_count: Number.NaN })).toBe("0/2");
    expect(taskProgress({ count: -3, completed_count: 1 })).toBe("");
    expect(taskProgress({ count: Number.NaN, completed_count: 1 })).toBe("");
  });
});
