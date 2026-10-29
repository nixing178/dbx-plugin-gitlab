import type { DiffLine } from "./diff";
import type { CommentPosition, DiffFile, DiffRefs } from "../types";

/**
 * 把 diff 行转换为 GitLab 行内评论的 position。
 * 新增/上下文行锚定新文件行号，删除行锚定旧文件行号。
 */
export function inlinePosition(
  file: Pick<DiffFile, "old_path" | "new_path">,
  line: DiffLine,
  refs: DiffRefs,
): CommentPosition | null {
  if (line.kind !== "add" && line.kind !== "del" && line.kind !== "context") {
    return null;
  }

  const position: CommentPosition = {
    base_sha: refs.base_sha,
    head_sha: refs.head_sha,
    start_sha: refs.start_sha,
    new_path: file.new_path || file.old_path,
    old_path: file.old_path || file.new_path,
    position_type: "text",
  };

  if (line.kind === "del") {
    if (line.old_line === null) return null;
    position.old_line = line.old_line;
    return position;
  }

  if (line.new_line === null) return null;
  position.new_line = line.new_line;
  return position;
}
