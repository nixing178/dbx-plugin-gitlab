export type StatusTone = "success" | "danger" | "warning" | "info" | "neutral" | "running";

const TONES: Record<string, StatusTone> = {
  success: "success",
  succeeded: "success",
  passed: "success",
  opened: "success",
  failed: "danger",
  failure: "danger",
  error: "danger",
  running: "running",
  in_progress: "running",
  preparing: "running",
  pending: "warning",
  waiting_for_resource: "warning",
  scheduled: "warning",
  locked: "warning",
  manual: "info",
  created: "info",
  merged: "info",
  closed: "neutral",
  canceled: "neutral",
  cancelled: "neutral",
  skipped: "neutral",
};

const TERMINAL = new Set([
  "success",
  "succeeded",
  "passed",
  "failed",
  "failure",
  "error",
  "canceled",
  "cancelled",
  "skipped",
  "merged",
  "closed",
]);

function normalize(status: string): string {
  return String(status ?? "").trim().toLowerCase();
}

export function statusTone(status: string): StatusTone {
  return TONES[normalize(status)] ?? "neutral";
}

export function statusKey(status: string): string {
  const normalized = normalize(status);
  return normalized ? `status_${normalized}` : "status_unknown";
}

export function isTerminal(status: string): boolean {
  return TERMINAL.has(normalize(status));
}

/**
 * 一个 stage 内多个作业的聚合状态。
 * 优先级：失败 > 运行中 > 全部成功/跳过 > 取第一个作业状态。
 */
export function aggregateStatus(statuses: string[]): string {
  const normalized = statuses.map(normalize);
  if (normalized.some((status) => status === "failed" || status === "failure" || status === "error")) {
    return "failed";
  }
  if (
    normalized.some(
      (status) => status === "running" || status === "in_progress" || status === "preparing",
    )
  ) {
    return "running";
  }
  if (
    normalized.length > 0 &&
    normalized.every((status) => ["success", "succeeded", "passed", "skipped"].includes(status))
  ) {
    return "success";
  }
  return normalized[0] ?? "pending";
}
