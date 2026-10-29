import type { Member } from "../types";

const LEVELS: Record<number, string> = {
  5: "role_minimal",
  10: "role_guest",
  20: "role_reporter",
  30: "role_developer",
  40: "role_maintainer",
  50: "role_owner",
};

/** 把 GitLab access_level 映射成 i18n key。 */
export function accessLevelKey(level: number): string {
  return LEVELS[level] ?? "role_unknown";
}

/**
 * 项目成员可分配的角色等级（高到低）。
 * Minimal Access(5) 只适用于群组成员，项目成员不能分配，故不在列表里；
 * 但 accessLevelKey(5) 仍保留，用于显示从组继承来的成员。
 */
export const PROJECT_MEMBER_LEVELS: number[] = [50, 40, 30, 20, 10];

/** 成员展示名：姓名优先，缺失时退回用户名。 */
export function memberDisplayName(member: Member): string {
  const name = (member.name ?? "").trim();
  return name || (member.username ?? "").trim();
}

/** 头像首字母：姓名首字大写，空值退回 ?。 */
export function memberInitial(member: Member): string {
  const source = memberDisplayName(member);
  return source ? source.slice(0, 1).toUpperCase() : "?";
}

const STATE_KEYS: Record<string, string> = {
  blocked: "memberStateBlocked",
  deactivated: "memberStateDeactivated",
};

/** 只有非活跃状态才需要徽标；active 返回空串。 */
export function memberStateKey(state: string): string {
  return STATE_KEYS[(state ?? "").trim().toLowerCase()] ?? "";
}

const SOURCE_KEYS: Record<string, string> = {
  direct: "memberSourceDirect",
  inherited: "memberSourceInherited",
};

/** 成员来源：direct（项目直接成员）/ inherited（从上层组继承），未知返回空串。 */
export function memberSourceKey(source: string): string {
  return SOURCE_KEYS[(source ?? "").trim().toLowerCase()] ?? "";
}

/** 当前登录账号在本项目的身份，用于判断成员操作权限。 */
export interface MemberViewer {
  id: number;
  accessLevel: number;
}

/**
 * 能否管理某个成员：至少维护者、等级不低于目标（同级可管理）、
 * 目标必须是直接成员、不能操作自己。
 */
export function canManageMember(member: Member, viewer: MemberViewer): boolean {
  if (!viewer.id || viewer.accessLevel < 40) return false;
  if (member.id === viewer.id) return false;
  if (member.source !== "direct") return false;
  return viewer.accessLevel >= member.access_level;
}

/**
 * 项目只剩这一位所有者时不能再降级或移除，否则项目会失去所有者。
 * 只统计已加载的成员，超过一页时可能误判，此时以 GitLab 的报错为准。
 */
export function isLastOwner(member: Member, members: Member[]): boolean {
  if (member.access_level < 50) return false;
  return members.filter((item) => item.access_level >= 50).length <= 1;
}

/** 改角色 = 有管理权限 + 非只读。 */
export function canEditMemberRole(
  member: Member,
  readOnly: boolean,
  viewer: MemberViewer,
): boolean {
  return !readOnly && canManageMember(member, viewer);
}

/** 邀请或改角色时可分配的角色：不高于自己的等级（含同级，与同级可管理一致）。 */
export function invitableRoles(viewerLevel: number, levels: number[]): number[] {
  return levels.filter((level) => viewerLevel >= level);
}

export type MemberInviteTarget = { kind: "email" | "username"; value: string };

/**
 * 邀请目标解析：含 @ 走邮箱邀请（POST /invitations），
 * 否则按用户名添加已有用户（POST /members）。
 */
export function memberInviteTarget(input: string): MemberInviteTarget | null {
  const value = (input ?? "").trim().replace(/^@/, "").trim();
  if (!value) return null;
  return { kind: value.includes("@") ? "email" : "username", value };
}

export type MemberSortKey = "role" | "name" | "expiry";
export type MemberSortDirection = "asc" | "desc";

/** 客户端筛选：姓名/用户名包含关键字（忽略大小写），role 为 0 表示全部角色。 */
export function filterMembers(members: Member[], query: string, role = 0): Member[] {
  const needle = (query ?? "").trim().toLowerCase();
  return members.filter((member) => {
    if (role && member.access_level !== role) return false;
    if (!needle) return true;
    const name = (member.name ?? "").toLowerCase();
    const username = (member.username ?? "").toLowerCase();
    return name.includes(needle) || username.includes(needle);
  });
}

function byName(a: Member, b: Member): number {
  return memberDisplayName(a).toLowerCase().localeCompare(memberDisplayName(b).toLowerCase());
}

function expiryTime(member: Member): number {
  return Date.parse(member.expires_at ?? "");
}

/** 排序：角色按等级、姓名忽略大小写、过期时间缺失恒定排在最后。 */
export function sortMembers(
  members: Member[],
  key: MemberSortKey,
  direction: MemberSortDirection,
): Member[] {
  const factor = direction === "asc" ? 1 : -1;
  const sorted = [...members];

  if (key === "expiry") {
    // 先分组再排序：没有过期时间的成员恒定排在最后，不随升降序翻转。
    const dated: Member[] = [];
    const undated: Member[] = [];
    for (const member of sorted) {
      (Number.isNaN(expiryTime(member)) ? undated : dated).push(member);
    }
    dated.sort((a, b) => (expiryTime(a) - expiryTime(b)) * factor || byName(a, b));
    undated.sort(byName);
    return [...dated, ...undated];
  }

  sorted.sort((a, b) => {
    if (key === "role") {
      const diff = (a.access_level ?? 0) - (b.access_level ?? 0);
      if (diff !== 0) return diff * factor;
    }
    return byName(a, b);
  });
  return sorted;
}
