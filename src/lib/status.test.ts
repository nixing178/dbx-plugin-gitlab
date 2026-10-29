import { describe, expect, it } from "vitest";
import { aggregateStatus, isTerminal, statusKey, statusTone } from "./status";

describe("statusTone", () => {
  it("maps pipeline and job states", () => {
    expect(statusTone("success")).toBe("success");
    expect(statusTone("failed")).toBe("danger");
    expect(statusTone("running")).toBe("running");
    expect(statusTone("pending")).toBe("warning");
    expect(statusTone("canceled")).toBe("neutral");
    expect(statusTone("skipped")).toBe("neutral");
    expect(statusTone("manual")).toBe("info");
  });

  it("maps merge request states", () => {
    expect(statusTone("merged")).toBe("info");
    expect(statusTone("opened")).toBe("success");
    expect(statusTone("closed")).toBe("neutral");
    expect(statusTone("locked")).toBe("warning");
  });

  it("is case insensitive and defaults to neutral", () => {
    expect(statusTone("SUCCESS")).toBe("success");
    expect(statusTone("something-else")).toBe("neutral");
    expect(statusTone("")).toBe("neutral");
  });
});

describe("statusKey", () => {
  it("returns i18n keys", () => {
    expect(statusKey("success")).toBe("status_success");
    expect(statusKey("running")).toBe("status_running");
    expect(statusKey("")).toBe("status_unknown");
  });
});

describe("isTerminal", () => {
  it("detects finished states", () => {
    expect(isTerminal("success")).toBe(true);
    expect(isTerminal("failed")).toBe(true);
    expect(isTerminal("canceled")).toBe(true);
    expect(isTerminal("running")).toBe(false);
    expect(isTerminal("pending")).toBe(false);
  });
});

describe("aggregateStatus", () => {
  it("prioritises failures over everything else", () => {
    expect(aggregateStatus(["success", "failed", "running"])).toBe("failed");
    expect(aggregateStatus(["running", "failed"])).toBe("failed");
  });

  it("reports running while any job is still going", () => {
    expect(aggregateStatus(["success", "running", "manual"])).toBe("running");
  });

  it("treats skipped jobs as done", () => {
    expect(aggregateStatus(["success", "skipped"])).toBe("success");
    expect(aggregateStatus(["skipped"])).toBe("success");
  });

  it("falls back to the first job status for mixed unfinished stages", () => {
    expect(aggregateStatus(["manual", "pending"])).toBe("manual");
    expect(aggregateStatus(["canceled", "success"])).toBe("canceled");
  });

  it("defaults to pending for an empty stage", () => {
    expect(aggregateStatus([])).toBe("pending");
  });
});
