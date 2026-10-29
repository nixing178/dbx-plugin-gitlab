export type DiffLineKind = "hunk" | "add" | "del" | "context" | "meta";

export interface DiffLine {
  kind: DiffLineKind;
  text: string;
  old_line: number | null;
  new_line: number | null;
}

export interface DiffStats {
  additions: number;
  deletions: number;
}

const HUNK_PATTERN = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/;

export function parseHunkHeader(
  line: string,
): { oldStart: number; oldCount: number; newStart: number; newCount: number } | null {
  const match = HUNK_PATTERN.exec(line);
  if (!match) return null;
  return {
    oldStart: Number(match[1]),
    oldCount: match[2] === undefined ? 1 : Number(match[2]),
    newStart: Number(match[3]),
    newCount: match[4] === undefined ? 1 : Number(match[4]),
  };
}

/**
 * 解析 GitLab 返回的 unified diff。
 * 行号按 hunk 头推进，新增行只有新行号、删除行只有旧行号。
 */
export function parseDiff(diff: string): DiffLine[] {
  if (!diff) return [];

  const lines = diff.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");
  const result: DiffLine[] = [];
  let oldLine = 0;
  let newLine = 0;

  for (const raw of lines) {
    if (raw.startsWith("\\")) {
      result.push({
        kind: "meta",
        text: raw.replace(/^\\\s?/, ""),
        old_line: null,
        new_line: null,
      });
      continue;
    }

    const hunk = parseHunkHeader(raw);
    if (hunk) {
      oldLine = hunk.oldStart;
      newLine = hunk.newStart;
      result.push({ kind: "hunk", text: raw, old_line: null, new_line: null });
      continue;
    }

    if (raw.startsWith("+")) {
      result.push({ kind: "add", text: raw.slice(1), old_line: null, new_line: newLine });
      newLine += 1;
      continue;
    }

    if (raw.startsWith("-")) {
      result.push({ kind: "del", text: raw.slice(1), old_line: oldLine, new_line: null });
      oldLine += 1;
      continue;
    }

    if (raw.startsWith(" ") || raw === "") {
      result.push({
        kind: "context",
        text: raw.startsWith(" ") ? raw.slice(1) : raw,
        old_line: oldLine,
        new_line: newLine,
      });
      oldLine += 1;
      newLine += 1;
      continue;
    }

    result.push({ kind: "meta", text: raw, old_line: null, new_line: null });
  }

  return result;
}

export function diffStats(diff: string): DiffStats {
  let additions = 0;
  let deletions = 0;
  for (const line of parseDiff(diff)) {
    if (line.kind === "add") additions += 1;
    if (line.kind === "del") deletions += 1;
  }
  return { additions, deletions };
}

export interface HunkRange {
  oldStart: number;
  oldCount: number;
  newStart: number;
  newCount: number;
}

/** 一个 hunk 及其正文；首个 block 的 header 可能为 null（例如二进制文件提示）。 */
export interface DiffBlock {
  header: DiffLine | null;
  range: HunkRange | null;
  lines: DiffLine[];
}

export function splitHunks(lines: DiffLine[]): DiffBlock[] {
  const blocks: DiffBlock[] = [];
  let current: DiffBlock = { header: null, range: null, lines: [] };

  for (const line of lines) {
    if (line.kind === "hunk") {
      if (current.header || current.lines.length) blocks.push(current);
      current = { header: line, range: parseHunkHeader(line.text), lines: [] };
      continue;
    }
    current.lines.push(line);
  }

  if (current.header || current.lines.length) blocks.push(current);
  return blocks;
}

/**
 * 用完整文件内容补齐 hunk 前后的上下文行。
 * 行号按新文件推进：hunk 之前的上下文与旧文件一致，因此 old_line 与 new_line 相同。
 */
export function expandContext(
  fileLines: string[],
  hunk: HunkRange,
  before = 10,
  after = 10,
): { before: DiffLine[]; after: DiffLine[] } {
  const result: { before: DiffLine[]; after: DiffLine[] } = { before: [], after: [] };
  if (!fileLines.length) return result;

  const beforeCount = Math.max(0, Math.min(before, hunk.newStart - 1));
  for (let offset = beforeCount; offset >= 1; offset -= 1) {
    const lineNo = hunk.newStart - offset;
    const text = fileLines[lineNo - 1];
    if (text === undefined) continue;
    result.before.push({ kind: "context", text, old_line: lineNo, new_line: lineNo });
  }

  const lastNewLine = hunk.newStart + hunk.newCount - 1;
  const available = fileLines.length - lastNewLine;
  const afterCount = Math.max(0, Math.min(after, available));
  for (let offset = 1; offset <= afterCount; offset += 1) {
    const lineNo = lastNewLine + offset;
    const text = fileLines[lineNo - 1];
    if (text === undefined) continue;
    result.after.push({ kind: "context", text, old_line: lineNo, new_line: lineNo });
  }

  return result;
}
