import { describe, expect, it } from "vitest";
import {
  DIFF_INITIAL_FILES,
  DIFF_PAGE_SIZE,
  diffInitialPages,
  diffMissingFiles,
} from "./diffPaging";

describe("diffInitialPages", () => {
  it("首屏至少拉一页", () => {
    expect(diffInitialPages(0)).toBe(1);
    expect(diffInitialPages(1)).toBe(1);
  });

  it("文件数在一页内只拉一页", () => {
    expect(diffInitialPages(DIFF_PAGE_SIZE)).toBe(1);
  });

  it("大 MR 首屏只拉够阈值就停", () => {
    expect(diffInitialPages(36)).toBe(Math.ceil(DIFF_INITIAL_FILES / DIFF_PAGE_SIZE));
    expect(diffInitialPages(500)).toBe(Math.ceil(DIFF_INITIAL_FILES / DIFF_PAGE_SIZE));
  });
});

describe("diffMissingFiles", () => {
  it("已加载足够时不显示缺失", () => {
    expect(diffMissingFiles(6, 6)).toBe(0);
    expect(diffMissingFiles(6, 8)).toBe(0);
  });

  it("只加载一部分时给出剩余数量", () => {
    expect(diffMissingFiles(36, 20)).toBe(16);
  });

  it("总数未知（0）时不报缺失，避免误导", () => {
    expect(diffMissingFiles(0, 0)).toBe(0);
    expect(diffMissingFiles(0, 3)).toBe(0);
  });
});
