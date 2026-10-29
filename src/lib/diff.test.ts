import { describe, expect, it } from "vitest";
import { diffStats, expandContext, parseDiff, parseHunkHeader, splitHunks } from "./diff";

const SAMPLE = [
  "@@ -1,4 +1,5 @@",
  " context one",
  "-removed line",
  "+added line",
  "+second added",
  " context two",
  "\\ No newline at end of file",
  "",
].join("\n");

describe("parseDiff", () => {
  it("assigns old and new line numbers", () => {
    const lines = parseDiff(SAMPLE);
    const hunk = lines.find((line) => line.kind === "hunk");
    expect(hunk?.text).toContain("@@ -1,4 +1,5 @@");

    const removed = lines.find((line) => line.kind === "del");
    expect(removed).toMatchObject({ text: "removed line", old_line: 2, new_line: null });

    const additions = lines.filter((line) => line.kind === "add");
    expect(additions[0]).toMatchObject({ text: "added line", old_line: null, new_line: 2 });
    expect(additions[1]).toMatchObject({ text: "second added", new_line: 3 });

    const contexts = lines.filter((line) => line.kind === "context");
    expect(contexts[0]).toMatchObject({ old_line: 1, new_line: 1 });
    expect(contexts[1]).toMatchObject({ old_line: 3, new_line: 4 });
  });

  it("returns an empty array for empty input", () => {
    expect(parseDiff("")).toEqual([]);
  });

  it("counts additions and deletions", () => {
    expect(diffStats(SAMPLE)).toEqual({ additions: 2, deletions: 1 });
  });

  it("marks no-newline notices as meta lines", () => {
    const meta = parseDiff(SAMPLE).find((line) => line.kind === "meta");
    expect(meta?.text).toContain("No newline");
    expect(meta?.old_line).toBeNull();
    expect(meta?.new_line).toBeNull();
  });
});

describe("parseHunkHeader", () => {
  it("parses counts", () => {
    expect(parseHunkHeader("@@ -12,7 +12,9 @@ func main() {")).toEqual({
      oldStart: 12,
      oldCount: 7,
      newStart: 12,
      newCount: 9,
    });
  });

  it("defaults counts to 1 when omitted", () => {
    expect(parseHunkHeader("@@ -1 +1 @@")).toEqual({
      oldStart: 1,
      oldCount: 1,
      newStart: 1,
      newCount: 1,
    });
  });

  it("rejects non hunk lines", () => {
    expect(parseHunkHeader("not a hunk")).toBeNull();
  });
});

describe("splitHunks", () => {
  const TWO_HUNKS = [
    "@@ -1,3 +1,3 @@",
    " one",
    "-two",
    "+TWO",
    "@@ -20,2 +20,3 @@",
    " twenty",
    "+twenty-one",
    " twenty-two",
  ].join("\n");

  it("keeps the preamble in its own block", () => {
    const blocks = splitHunks(parseDiff("Binary files a.png and b.png differ"));
    expect(blocks).toHaveLength(1);
    expect(blocks[0].header).toBeNull();
    expect(blocks[0].range).toBeNull();
    expect(blocks[0].lines[0].kind).toBe("meta");
  });

  it("groups every hunk with its own body", () => {
    const blocks = splitHunks(parseDiff(TWO_HUNKS));
    expect(blocks).toHaveLength(2);
    expect(blocks[0].range).toEqual({ oldStart: 1, oldCount: 3, newStart: 1, newCount: 3 });
    expect(blocks[0].lines.map((line) => line.kind)).toEqual(["context", "del", "add"]);
    expect(blocks[1].range?.newStart).toBe(20);
    expect(blocks[1].lines.map((line) => line.kind)).toEqual(["context", "add", "context"]);
  });
});

describe("expandContext", () => {
  const file = ["l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10"];

  it("numbers expanded lines continuously with the hunk", () => {
    const result = expandContext(file, { oldStart: 5, oldCount: 2, newStart: 5, newCount: 2 }, 2, 2);
    expect(result.before.map((line) => [line.text, line.new_line])).toEqual([
      ["l3", 3],
      ["l4", 4],
    ]);
    expect(result.after.map((line) => [line.text, line.new_line])).toEqual([
      ["l7", 7],
      ["l8", 8],
    ]);
    expect(result.before.every((line) => line.kind === "context")).toBe(true);
  });

  it("clamps at the start of the file", () => {
    const result = expandContext(file, { oldStart: 2, oldCount: 1, newStart: 2, newCount: 1 }, 5, 0);
    expect(result.before).toHaveLength(1);
    expect(result.before[0]).toMatchObject({ text: "l1", old_line: 1, new_line: 1 });
  });

  it("clamps at the end of the file", () => {
    const result = expandContext(file, { oldStart: 9, oldCount: 2, newStart: 9, newCount: 2 }, 0, 5);
    expect(result.after).toEqual([]);
  });

  it("handles pure deletions where newCount is zero", () => {
    const result = expandContext(file, { oldStart: 5, oldCount: 2, newStart: 6, newCount: 0 }, 0, 1);
    expect(result.after).toHaveLength(1);
    expect(result.after[0]).toMatchObject({ text: "l6", new_line: 6 });
  });

  it("returns empty arrays when nothing is requested or the file is unknown", () => {
    const hunk = { oldStart: 5, oldCount: 1, newStart: 5, newCount: 1 };
    expect(expandContext(file, hunk, 0, 0)).toEqual({ before: [], after: [] });
    expect(expandContext([], hunk, 10, 10)).toEqual({ before: [], after: [] });
  });
});
