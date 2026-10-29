export type LogLevel = "error" | "warn" | "info" | "plain";
export type LogFilter = "all" | "problems";

export interface LogLine {
  number: number;
  text: string;
  level: LogLevel;
}

const ANSI_PATTERN = /\u001b\[[0-9;]*[A-Za-z]|\u001b\][^\u0007]*(?:\u0007|\u001b\\)/g;

const ERROR_PATTERNS = [
  /\b(?:ERROR|FATAL|SEVERE)\b/,
  /\b(?:BUILD FAILED|FAILURE|ABORTED)\b/,
  /Exception\b/,
  /Caused by:/,
  /\berror:\s/i,
  /##\[error\]/i,
];

const WARN_PATTERNS = [/\b(?:WARN|WARNING)\b/i, /\bwarning:\s/i, /\bDEPRECATED\b/i];

const INFO_PATTERN = /^\s*\[?(?:INFO|DEBUG|TRACE)\]?(?:\s|:)/i;

export function stripAnsi(text: string): string {
  return text.includes("\u001b") ? text.replace(ANSI_PATTERN, "") : text;
}

export function classifyLine(text: string): LogLevel {
  if (!text) return "plain";
  for (const pattern of ERROR_PATTERNS) {
    if (pattern.test(text)) return "error";
  }
  for (const pattern of WARN_PATTERNS) {
    if (pattern.test(text)) return "warn";
  }
  if (INFO_PATTERN.test(text)) return "info";
  return "plain";
}

export function toLines(text: string): LogLine[] {
  if (!text) return [];

  const raw = text.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");
  return raw.map((line, index) => {
    const clean = stripAnsi(line);
    return { number: index + 1, text: clean, level: classifyLine(clean) };
  });
}

export function filterLines(lines: LogLine[], mode: LogFilter, query: string): LogLine[] {
  const needle = query.trim().toLowerCase();
  return lines.filter((line) => {
    if (mode === "problems" && line.level !== "error" && line.level !== "warn") {
      return false;
    }
    if (needle && !line.text.toLowerCase().includes(needle)) {
      return false;
    }
    return true;
  });
}

export function countProblems(lines: LogLine[]): { errors: number; warnings: number } {
  let errors = 0;
  let warnings = 0;
  for (const line of lines) {
    if (line.level === "error") errors += 1;
    else if (line.level === "warn") warnings += 1;
  }
  return { errors, warnings };
}
