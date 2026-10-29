/**
 * 分支名校验，覆盖 GitLab/Git 拒绝的常见写法。
 * 只做前端提示；GitLab 返回的 400 仍以错误横幅为准。
 */
export type BranchNameError = "empty" | "whitespace" | "invalid";

/** Git 引用禁止的字符：~ ^ : ? * [ \ */
const ILLEGAL_CHARS = /[~^:?*\[\\]/;
const CONTROL_CHARS = /[\u0000-\u001f\u007f]/;

export function validateBranchName(raw: string): BranchNameError | null {
  const name = (raw ?? "").trim();
  if (!name) return "empty";
  if (/\s/.test(name)) return "whitespace";

  if (
    name === "@" ||
    name.startsWith("-") ||
    name.startsWith("/") ||
    name.endsWith("/") ||
    name.endsWith(".") ||
    name.includes("..") ||
    name.includes("//") ||
    name.includes("@{") ||
    ILLEGAL_CHARS.test(name) ||
    CONTROL_CHARS.test(name)
  ) {
    return "invalid";
  }

  // 逐段检查：不能以 . 开头（如 .hidden），不能以 .lock 结尾。
  const parts = name.split("/");
  if (parts.some((part) => part.startsWith(".") || part.endsWith(".lock"))) return "invalid";

  return null;
}
