import { describe, expect, it } from "vitest";
import { projectGate } from "./projectGate";

describe("projectGate", () => {
  it("stays neutral on the project list route", () => {
    // 回归：空路径 + 空 missing 曾判定为 missing，把项目列表整页顶掉
    expect(projectGate("", "", false)).toBe("ok");
  });

  it("reports loading while a project detail is being fetched", () => {
    expect(projectGate("demo/app", "", false)).toBe("loading");
    expect(projectGate("demo/app", "demo/other", false)).toBe("loading");
  });

  it("reports missing only for the path that failed", () => {
    expect(projectGate("demo/app", "demo/app", false)).toBe("missing");
  });

  it("is ok once the project is cached", () => {
    expect(projectGate("demo/app", "", true)).toBe("ok");
    expect(projectGate("demo/app", "demo/app", true)).toBe("ok");
  });
});
