import { describe, expect, it } from "vitest";
import { classifyLine, countProblems, filterLines, stripAnsi, toLines } from "./logLines";

describe("classifyLine", () => {
  it("detects errors", () => {
    expect(classifyLine("ERROR: build failed")).toBe("error");
    expect(classifyLine("java.lang.NullPointerException")).toBe("error");
    expect(classifyLine("##[error]Process completed with exit code 1")).toBe("error");
    expect(classifyLine("BUILD FAILED")).toBe("error");
    expect(classifyLine("error: cannot find module")).toBe("error");
  });

  it("detects warnings", () => {
    expect(classifyLine("WARN something odd")).toBe("warn");
    expect(classifyLine("warning: unused variable")).toBe("warn");
    expect(classifyLine("DEPRECATED api")).toBe("warn");
  });

  it("detects info", () => {
    expect(classifyLine("[INFO] starting")).toBe("info");
    expect(classifyLine("DEBUG payload")).toBe("info");
  });

  it("defaults to plain", () => {
    expect(classifyLine("compiling 42 files")).toBe("plain");
    expect(classifyLine("")).toBe("plain");
  });
});

describe("stripAnsi", () => {
  it("removes color codes", () => {
    expect(stripAnsi("\u001b[31mred\u001b[0m")).toBe("red");
    expect(stripAnsi("\u001b[1;32mok\u001b[0m done")).toBe("ok done");
  });

  it("leaves plain text untouched", () => {
    expect(stripAnsi("plain")).toBe("plain");
  });
});

describe("toLines", () => {
  it("numbers lines from 1", () => {
    expect(toLines("a\nb")).toEqual([
      { number: 1, text: "a", level: "plain" },
      { number: 2, text: "b", level: "plain" },
    ]);
  });

  it("drops the trailing empty line", () => {
    expect(toLines("a\n")).toHaveLength(1);
  });

  it("returns an empty array for empty input", () => {
    expect(toLines("")).toEqual([]);
  });

  it("strips ansi before classifying", () => {
    expect(toLines("\u001b[31mERROR\u001b[0m boom")[0]).toMatchObject({
      text: "ERROR boom",
      level: "error",
    });
  });
});

describe("filterLines", () => {
  const lines = toLines("ok\nERROR boom\nwarning: hmm\ninfo");

  it("keeps only problems", () => {
    expect(filterLines(lines, "problems", "").map((line) => line.number)).toEqual([2, 3]);
  });

  it("filters by query case-insensitively", () => {
    expect(filterLines(lines, "all", "BOOM").map((line) => line.number)).toEqual([2]);
  });

  it("combines problems and query", () => {
    expect(filterLines(lines, "problems", "hmm").map((line) => line.number)).toEqual([3]);
  });

  it("returns everything when mode is all and query is empty", () => {
    expect(filterLines(lines, "all", "")).toHaveLength(4);
  });
});

describe("countProblems", () => {
  it("counts errors and warnings", () => {
    expect(countProblems(toLines("ERROR a\nwarning b\nok"))).toEqual({
      errors: 1,
      warnings: 1,
    });
  });

  it("returns zeros for a clean log", () => {
    expect(countProblems(toLines("ok"))).toEqual({ errors: 0, warnings: 0 });
  });
});
