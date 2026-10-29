import { describe, expect, it } from "vitest";
import { hasStarred, setStarred } from "./starredPaths";

describe("starredPaths", () => {
  it("detects a starred path", () => {
    expect(hasStarred(["a/b", "c/d"], "c/d")).toBe(true);
    expect(hasStarred(["a/b"], "a/c")).toBe(false);
    expect(hasStarred([], "a/b")).toBe(false);
  });

  it("adds a path only once", () => {
    expect(setStarred(["a/b"], "c/d", true)).toEqual(["a/b", "c/d"]);
    expect(setStarred(["a/b", "c/d"], "c/d", true)).toEqual(["a/b", "c/d"]);
  });

  it("removes a path when unstarred", () => {
    expect(setStarred(["a/b", "c/d"], "c/d", false)).toEqual(["a/b"]);
    expect(setStarred(["a/b"], "c/d", false)).toEqual(["a/b"]);
  });

  it("does not mutate the input array", () => {
    const input = ["a/b"];
    const next = setStarred(input, "c/d", true);
    expect(input).toEqual(["a/b"]);
    expect(next).not.toBe(input);
  });
});
