import { describe, expect, it } from "vitest";
import type { ProjectNav } from "./nav";
import { isPinned, normalizePins, splitProjectNav, togglePin, PROJECT_NAV_ORDER } from "./navPins";

describe("normalizePins", () => {
  it("drops unknown ids, duplicates and keeps the pin order", () => {
    expect(normalizePins(["mrs", "nope", "code", "mrs", 42, null])).toEqual(["mrs", "code"]);
  });

  it("falls back to an empty list for garbage input", () => {
    expect(normalizePins(undefined)).toEqual([]);
    expect(normalizePins(null)).toEqual([]);
    expect(normalizePins("mrs")).toEqual([]);
    expect(normalizePins({ pinned: ["mrs"] })).toEqual([]);
  });
});

describe("isPinned", () => {
  it("only matches ids present in the pin list", () => {
    expect(isPinned(["mrs"], "mrs")).toBe(true);
    expect(isPinned(["mrs"], "code")).toBe(false);
    expect(isPinned([], "code")).toBe(false);
  });
});

describe("togglePin", () => {
  it("appends a new pin at the end", () => {
    expect(togglePin(["mrs"], "code")).toEqual(["mrs", "code"]);
  });

  it("removes an existing pin and keeps the rest in order", () => {
    expect(togglePin(["mrs", "code", "ci"], "code")).toEqual(["mrs", "ci"]);
  });

  it("returns a new array instead of mutating the input", () => {
    const pins: ProjectNav[] = ["mrs"];
    const next = togglePin(pins, "code");
    expect(next).not.toBe(pins);
    expect(pins).toEqual(["mrs"]);
  });

  it("ignores pinning an id twice in the stored list", () => {
    expect(togglePin(["mrs", "mrs"], "mrs")).toEqual([]);
  });
});

describe("splitProjectNav", () => {
  it("keeps the pin order and leaves the rest in canonical order", () => {
    expect(splitProjectNav(["ci", "issues"])).toEqual({
      pinned: ["ci", "issues"],
      rest: ["code", "commits", "branches", "mrs", "members"],
    });
  });

  it("returns the full nav as rest when nothing is pinned", () => {
    expect(splitProjectNav([])).toEqual({ pinned: [], rest: PROJECT_NAV_ORDER });
  });

  it("drops unknown pins and never duplicates an item", () => {
    const { pinned, rest } = splitProjectNav(["nope", "code", "code"] as ProjectNav[]);
    expect(pinned).toEqual(["code"]);
    expect(rest).toEqual(["commits", "branches", "issues", "mrs", "ci", "members"]);
  });

  it("leaves rest empty when everything is pinned", () => {
    expect(splitProjectNav([...PROJECT_NAV_ORDER])).toEqual({
      pinned: [...PROJECT_NAV_ORDER],
      rest: [],
    });
  });
});
