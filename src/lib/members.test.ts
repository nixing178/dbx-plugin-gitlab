import { describe, expect, it } from "vitest";
import { accessLevelKey } from "./members";

describe("accessLevelKey", () => {
  it("maps GitLab access levels", () => {
    expect(accessLevelKey(5)).toBe("role_minimal");
    expect(accessLevelKey(10)).toBe("role_guest");
    expect(accessLevelKey(20)).toBe("role_reporter");
    expect(accessLevelKey(30)).toBe("role_developer");
    expect(accessLevelKey(40)).toBe("role_maintainer");
    expect(accessLevelKey(50)).toBe("role_owner");
  });

  it("falls back for unknown levels", () => {
    expect(accessLevelKey(0)).toBe("role_unknown");
    expect(accessLevelKey(99)).toBe("role_unknown");
    expect(accessLevelKey(Number.NaN)).toBe("role_unknown");
  });
});
