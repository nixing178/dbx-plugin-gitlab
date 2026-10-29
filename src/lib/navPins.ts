import type { ProjectNav } from "./nav";

/** 项目内二级导航的规范顺序，与 AppSidebar 的渲染顺序一致。 */
export const PROJECT_NAV_ORDER: ProjectNav[] = [
  "code",
  "commits",
  "branches",
  "issues",
  "mrs",
  "ci",
  "members",
];

const KNOWN = new Set<string>(PROJECT_NAV_ORDER);

export interface ProjectNavGroup {
  /** 已钉选（置顶），顺序即用户钉选的先后顺序。 */
  pinned: ProjectNav[];
  /** 未钉选，保持规范顺序。 */
  rest: ProjectNav[];
}

/**
 * 反序列化钉选列表：丢掉未知项（升级后退役的入口）与重复项，
 * 其余保持用户钉选时的先后顺序。
 */
export function normalizePins(raw: unknown): ProjectNav[] {
  if (!Array.isArray(raw)) return [];
  const seen = new Set<ProjectNav>();
  const pins: ProjectNav[] = [];
  for (const item of raw) {
    if (typeof item !== "string" || !KNOWN.has(item)) continue;
    const id = item as ProjectNav;
    if (seen.has(id)) continue;
    seen.add(id);
    pins.push(id);
  }
  return pins;
}

export function isPinned(pins: ProjectNav[], id: ProjectNav): boolean {
  return pins.includes(id);
}

/** 钉选切换：新钉选追加到末尾（后钉的排在已钉选末尾），取消钉选直接移除。 */
export function togglePin(pins: ProjectNav[], id: ProjectNav): ProjectNav[] {
  return isPinned(pins, id) ? pins.filter((item) => item !== id) : [...pins, id];
}

/** 左栏分组：已钉选按钉选顺序，其余按规范顺序，两边不重复。 */
export function splitProjectNav(
  pins: ProjectNav[],
  order: ProjectNav[] = PROJECT_NAV_ORDER,
): ProjectNavGroup {
  const pinned = normalizePins(pins).filter((id) => order.includes(id));
  const pinnedSet = new Set(pinned);
  return { pinned, rest: order.filter((id) => !pinnedSet.has(id)) };
}
