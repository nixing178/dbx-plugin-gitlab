import { describe, expect, it } from "vitest";
import {
  commitBody,
  dateInputValue,
  formatBytes,
  formatDate,
  formatDateTime,
  formatDuration,
  relativeTime,
} from "./format";

const NOW = Date.parse("2026-10-23T12:00:00Z");

describe("relativeTime", () => {
  it("handles recent timestamps", () => {
    expect(relativeTime("2026-10-23T11:59:40Z", "zh-CN", NOW)).toBe("刚刚");
    expect(relativeTime("2026-10-23T11:59:40Z", "en", NOW)).toBe("just now");
  });

  it("handles minutes, hours and days", () => {
    expect(relativeTime("2026-10-23T11:55:00Z", "zh-CN", NOW)).toBe("5 分钟前");
    expect(relativeTime("2026-10-23T09:00:00Z", "zh-CN", NOW)).toBe("3 小时前");
    expect(relativeTime("2026-10-20T12:00:00Z", "zh-CN", NOW)).toBe("3 天前");
    expect(relativeTime("2026-10-23T09:00:00Z", "en", NOW)).toBe("3 hours ago");
  });

  it("falls back to an empty string for invalid input", () => {
    expect(relativeTime("", "zh-CN", NOW)).toBe("");
    expect(relativeTime("not-a-date", "zh-CN", NOW)).toBe("");
  });
});

describe("formatDateTime", () => {
  it("returns a non-empty local string", () => {
    expect(formatDateTime("2026-10-23T09:00:00Z", "zh-CN")).not.toBe("");
  });

  it("returns an empty string for invalid input", () => {
    expect(formatDateTime("", "zh-CN")).toBe("");
  });
});

describe("formatDate", () => {
  it("只保留日期部分，不带时分", () => {
    const text = formatDate("2026-10-23T09:00:00Z", "en-US");
    expect(text).toMatch(/2026/);
    expect(text).not.toMatch(/:/);
  });

  it("returns an empty string for invalid input", () => {
    expect(formatDate("", "zh-CN")).toBe("");
    expect(formatDate("not-a-date", "zh-CN")).toBe("");
  });
});

describe("dateInputValue", () => {
  it("截出 YYYY-MM-DD 供日期输入框使用", () => {
    expect(dateInputValue("2026-10-29")).toBe("2026-10-29");
    expect(dateInputValue("2026-10-29T10:20:30Z")).toBe("2026-10-29");
  });

  it("空值或非法值返回空串", () => {
    expect(dateInputValue("")).toBe("");
    expect(dateInputValue("   ")).toBe("");
    expect(dateInputValue("not-a-date")).toBe("");
  });
});

describe("formatBytes", () => {
  it("formats sizes", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(1024)).toBe("1.0 KB");
    expect(formatBytes(1536)).toBe("1.5 KB");
    expect(formatBytes(1024 * 1024)).toBe("1.0 MB");
  });
});

describe("commitBody", () => {
  it("drops the title line and keeps the description", () => {
    const title = "Merge branch 'feature-sample-export' into 'master'";
    expect(commitBody(`${title}\n\nSee merge request 1234`, title)).toBe("See merge request 1234");
  });

  it("collapses a title-only message to an empty body", () => {
    expect(commitBody("fix: 示例冲突校验", "fix: 示例冲突校验")).toBe("");
    expect(commitBody("", "fix: 示例冲突校验")).toBe("");
    expect(commitBody("   ", "fix: 示例冲突校验")).toBe("");
  });

  it("cleans CRLF and extra blank lines around the body", () => {
    expect(commitBody("feat: 示例信息导出\r\n\r\n\r\n- 模板\r\n- 校验\r\n", "feat: 示例信息导出")).toBe(
      "- 模板\n- 校验",
    );
  });

  it("falls back to the remaining lines when the title differs", () => {
    expect(commitBody("fix: 空指针\n\n复现：导入重复数据", "fix: 空指针（已截断）")).toBe(
      "复现：导入重复数据",
    );
  });
});

describe("formatDuration", () => {
  it("formats seconds", () => {
    expect(formatDuration(45)).toBe("45s");
    expect(formatDuration(90)).toBe("1m 30s");
    expect(formatDuration(3661)).toBe("1h 1m 1s");
  });

  it("handles invalid values", () => {
    expect(formatDuration(-1)).toBe("-");
    expect(formatDuration(Number.NaN)).toBe("-");
  });
});
