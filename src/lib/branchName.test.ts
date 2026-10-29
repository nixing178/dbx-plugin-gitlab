import { describe, expect, it } from "vitest";
import { validateBranchName } from "./branchName";

describe("validateBranchName", () => {
  it("accepts regular names, slashes and non-ASCII", () => {
    for (const name of [
      "test",
      "master",
      "feature/user2-20261028-分支筛选",
      "release/1.2.3",
      "a_b-c.d",
      "  feature/x  ",
    ]) {
      expect(validateBranchName(name)).toBeNull();
    }
  });

  it("separates empty from whitespace and illegal refs", () => {
    expect(validateBranchName("")).toBe("empty");
    expect(validateBranchName("   ")).toBe("empty");
    expect(validateBranchName("feature new")).toBe("whitespace");
    expect(validateBranchName("feature\tnew")).toBe("whitespace");
  });

  it("rejects the git ref rules GitLab enforces", () => {
    for (const name of [
      "-feature",
      "feature..x",
      "feature~1",
      "feature^x",
      "feature:x",
      "feature?x",
      "feature*x",
      "feature[x",
      "feature\\x",
      "/feature",
      "feature/",
      "feature//x",
      "feature.lock",
      "feature.",
      "@",
      "feature@{1}",
      "feature/.hidden",
    ]) {
      expect(validateBranchName(name)).toBe("invalid");
    }
  });
});
