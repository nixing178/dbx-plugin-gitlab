/**
 * 详情页的项目加载门禁。
 * 关键边界：项目列表路由下 path 与 missing 都是空串，不能判定为 missing。
 */
export type ProjectGate = "ok" | "loading" | "missing";

export function projectGate(path: string, missingPath: string, loaded: boolean): ProjectGate {
  if (!path || loaded) return "ok";
  return missingPath === path ? "missing" : "loading";
}
