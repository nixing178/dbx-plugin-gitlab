import { describe, expect, it } from "vitest";
import { fileExtension, fileName, joinRepoPath, parentPath, shortenMiddle } from "./paths";

describe("fileName", () => {
  it("returns the last segment", () => {
    expect(fileName("src/main.go")).toBe("main.go");
    expect(fileName("main.go")).toBe("main.go");
    expect(fileName("")).toBe("");
  });
});

describe("fileExtension", () => {
  it("returns a lowercase extension", () => {
    expect(fileExtension("Main.GO")).toBe("go");
    expect(fileExtension("a/b/readme.md")).toBe("md");
    expect(fileExtension("Makefile")).toBe("");
    expect(fileExtension(".gitignore")).toBe("");
  });
});

describe("joinRepoPath", () => {
  it("joins without duplicate slashes", () => {
    expect(joinRepoPath("src", "main.go")).toBe("src/main.go");
    expect(joinRepoPath("", "main.go")).toBe("main.go");
    expect(joinRepoPath("/src/", "/main.go")).toBe("src/main.go");
  });
});

describe("parentPath", () => {
  it("returns the parent directory", () => {
    expect(parentPath("src/core/a.go")).toBe("src/core");
    expect(parentPath("a.go")).toBe("");
    expect(parentPath("")).toBe("");
  });
});

describe("shortenMiddle", () => {
  it("keeps both ends", () => {
    expect(shortenMiddle("abcdefghij", 6)).toBe("abc...hij");
  });

  it("returns the input when it already fits", () => {
    expect(shortenMiddle("abc", 6)).toBe("abc");
    expect(shortenMiddle("abcdef", 6)).toBe("abcdef");
  });
});
