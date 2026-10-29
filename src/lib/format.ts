function isChinese(locale: string): boolean {
  return String(locale ?? "").toLowerCase().startsWith("zh");
}

function parse(iso: string): number {
  if (!iso) return Number.NaN;
  return Date.parse(iso);
}

export function relativeTime(iso: string, locale: string, now = Date.now()): string {
  const time = parse(iso);
  if (Number.isNaN(time)) return "";

  const zh = isChinese(locale);
  const seconds = Math.round((now - time) / 1000);

  if (seconds < 60) return zh ? "刚刚" : "just now";

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return zh ? `${minutes} 分钟前` : `${minutes} minute${minutes === 1 ? "" : "s"} ago`;
  }

  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return zh ? `${hours} 小时前` : `${hours} hour${hours === 1 ? "" : "s"} ago`;
  }

  const days = Math.floor(hours / 24);
  if (days < 30) {
    return zh ? `${days} 天前` : `${days} day${days === 1 ? "" : "s"} ago`;
  }

  return formatDateTime(iso, locale);
}

export function formatDateTime(iso: string, locale: string): string {
  const time = parse(iso);
  if (Number.isNaN(time)) return "";
  return new Date(time).toLocaleString(isChinese(locale) ? "zh-CN" : "en-US", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** 只格式化日期（用于成员过期时间这类不需要时分的字段）。 */
export function formatDate(iso: string, locale: string): string {
  const time = parse(iso);
  if (Number.isNaN(time)) return "";
  return new Date(time).toLocaleDateString(isChinese(locale) ? "zh-CN" : "en-US", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  });
}

/**
 * 提交正文：GitLab 的 `message` 首行就是 `title`（可能被实例截断），
 * 详情页标题已展示过一次，正文只保留其余部分。
 */
export function commitBody(message: string, title: string): string {
  const text = (message ?? "").replace(/\r\n/g, "\n").trim();
  if (!text) return "";

  const firstBreak = text.indexOf("\n");
  if (firstBreak === -1) return "";

  let rest = text.slice(firstBreak + 1);
  const heading = title.trim();
  // 个别实例会在这里再重复一次完整标题，去掉这段冗余。
  if (heading && rest.trimStart().startsWith(heading)) {
    rest = rest.trimStart().slice(heading.length);
  }
  return rest.trim();
}

/** 把 expires_at 之类的 ISO 时间转成 <input type="date"> 的值；空值/非法值返回空串。 */
export function dateInputValue(iso: string): string {
  const text = (iso ?? "").trim();
  if (!text) return "";
  const match = text.match(/^(\d{4}-\d{2}-\d{2})/);
  return match ? match[1] : "";
}

export function formatBytes(size: number): string {
  if (!Number.isFinite(size) || size < 0) return "-";
  if (size < 1024) return `${Math.floor(size)} B`;

  const units = ["KB", "MB", "GB", "TB"];
  let value = size / 1024;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value.toFixed(1)} ${units[unitIndex]}`;
}

export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "-";

  const total = Math.floor(seconds);
  if (total < 60) return `${total}s`;

  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const rest = total % 60;

  const parts: string[] = [];
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);
  if (rest > 0 || parts.length === 0) parts.push(`${rest}s`);
  return parts.join(" ");
}
