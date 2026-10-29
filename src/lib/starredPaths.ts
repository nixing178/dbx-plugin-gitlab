/** 已加星项目路径集合的纯函数操作；列表页用它判断每行星标状态。 */
export function hasStarred(paths: readonly string[], path: string): boolean {
  return paths.includes(path);
}

export function setStarred(paths: readonly string[], path: string, starred: boolean): string[] {
  const exists = paths.includes(path);
  if (starred === exists) return [...paths];
  return starred ? [...paths, path] : paths.filter((item) => item !== path);
}
