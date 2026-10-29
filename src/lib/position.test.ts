import { describe, expect, it } from "vitest";
import { inlinePosition } from "./position";
import type { DiffLine } from "./diff";

const refs = { base_sha: "base1", head_sha: "head2", start_sha: "start3" };
const file = { old_path: "src/a.go", new_path: "src/a.go" };

function line(kind: DiffLine["kind"], oldLine: number | null, newLine: number | null): DiffLine {
  return { kind, text: "", old_line: oldLine, new_line: newLine };
}

describe("inlinePosition", () => {
  it("anchors an added line on the new side", () => {
    expect(inlinePosition(file, line("add", null, 12), refs)).toEqual({
      base_sha: "base1",
      head_sha: "head2",
      start_sha: "start3",
      new_path: "src/a.go",
      old_path: "src/a.go",
      position_type: "text",
      new_line: 12,
    });
  });

  it("anchors a deleted line on the old side", () => {
    const position = inlinePosition(file, line("del", 7, null), refs);
    expect(position?.old_line).toBe(7);
    expect(position?.new_line).toBeUndefined();
  });

  it("anchors a context line on the new side", () => {
    const position = inlinePosition(file, line("context", 5, 6), refs);
    expect(position?.new_line).toBe(6);
    expect(position?.old_line).toBeUndefined();
  });

  it("returns null for hunk and meta lines", () => {
    expect(inlinePosition(file, line("hunk", null, null), refs)).toBeNull();
    expect(inlinePosition(file, line("meta", null, null), refs)).toBeNull();
  });

  it("returns null when the chosen side has no line number", () => {
    expect(inlinePosition(file, line("add", null, null), refs)).toBeNull();
    expect(inlinePosition(file, line("del", null, null), refs)).toBeNull();
  });

  it("falls back to the other path when one side is empty", () => {
    const renamed = { old_path: "old.go", new_path: "" };
    const position = inlinePosition(renamed, line("add", null, 1), refs);
    expect(position?.new_path).toBe("old.go");
    expect(position?.old_path).toBe("old.go");
  });
});
