export function fileName(path: string): string {
  const trimmed = String(path ?? "").replace(/\/+$/, "");
  if (!trimmed) return "";
  const index = trimmed.lastIndexOf("/");
  return index >= 0 ? trimmed.slice(index + 1) : trimmed;
}

export function fileExtension(path: string): string {
  const name = fileName(path);
  const index = name.lastIndexOf(".");
  if (index <= 0) return "";
  return name.slice(index + 1).toLowerCase();
}

export function joinRepoPath(dir: string, name: string): string {
  const left = String(dir ?? "").replace(/^\/+|\/+$/g, "");
  const right = String(name ?? "").replace(/^\/+|\/+$/g, "");
  if (!left) return right;
  if (!right) return left;
  return `${left}/${right}`;
}

export function parentPath(path: string): string {
  const trimmed = String(path ?? "").replace(/^\/+|\/+$/g, "");
  const index = trimmed.lastIndexOf("/");
  return index > 0 ? trimmed.slice(0, index) : "";
}

/** 中间省略，保留首尾共 max 个字符。 */
export function shortenMiddle(text: string, max: number): string {
  const value = String(text ?? "");
  if (max <= 0 || value.length <= max + 3) return value;

  const head = Math.ceil(max / 2);
  const tail = Math.floor(max / 2);
  return `${value.slice(0, head)}...${value.slice(value.length - tail)}`;
}
