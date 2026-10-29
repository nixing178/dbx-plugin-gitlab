import { describe, expect, it } from "vitest";
import type { Member } from "../types";
import {
  accessLevelKey,
  canEditMemberRole,
  canManageMember,
  filterMembers,
  invitableRoles,
  isLastOwner,
  memberDisplayName,
  memberInitial,
  memberInviteTarget,
  memberSourceKey,
  memberStateKey,
  PROJECT_MEMBER_LEVELS,
  sortMembers,
} from "./members";

function member(partial: Partial<Member> & { id: number }): Member {
  return {
    username: `user${partial.id}`,
    name: `User ${partial.id}`,
    state: "active",
    access_level: 30,
    avatar_url: "",
    web_url: `https://gitlab.example.com/u${partial.id}`,
    expires_at: null,
    ...partial,
  };
}

const team: Member[] = [
  member({ id: 1, username: "user1", name: "User One", access_level: 40 }),
  member({ id: 2, username: "user2", name: "User Two", access_level: 50 }),
  member({ id: 3, username: "user3", name: "User Three", access_level: 30 }),
  member({
    id: 4,
    username: "user4",
    name: "User Four",
    access_level: 20,
    expires_at: "2026-10-29T00:00:00Z",
  }),
  member({ id: 5, username: "user5", name: "User Five", access_level: 30, state: "blocked" }),
];

const ids = (list: Member[]): number[] => list.map((item) => item.id);

describe("PROJECT_MEMBER_LEVELS", () => {
  it("项目成员不能分配 Minimal Access（该角色仅群组成员可用）", () => {
    expect(PROJECT_MEMBER_LEVELS).not.toContain(5);
    expect(PROJECT_MEMBER_LEVELS).toEqual([50, 40, 30, 20, 10]);
  });
});

describe("accessLevelKey", () => {
  it("maps GitLab access levels", () => {
    expect(accessLevelKey(5)).toBe("role_minimal");
    expect(accessLevelKey(10)).toBe("role_guest");
    expect(accessLevelKey(20)).toBe("role_reporter");
    expect(accessLevelKey(30)).toBe("role_developer");
    expect(accessLevelKey(40)).toBe("role_maintainer");
    expect(accessLevelKey(50)).toBe("role_owner");
  });

  it("falls back for unknown levels", () => {
    expect(accessLevelKey(0)).toBe("role_unknown");
    expect(accessLevelKey(99)).toBe("role_unknown");
    expect(accessLevelKey(Number.NaN)).toBe("role_unknown");
  });
});

describe("memberDisplayName / memberInitial", () => {
  it("优先用姓名，缺失时退回用户名", () => {
    expect(memberDisplayName(member({ id: 1, name: "User Two" }))).toBe("User Two");
    expect(memberDisplayName(member({ id: 2, name: "  ", username: "user3" }))).toBe("user3");
  });

  it("头像首字母取姓名首字，空值退回 ?", () => {
    expect(memberInitial(member({ id: 1, name: "user2" }))).toBe("U");
    expect(memberInitial(member({ id: 2, name: "", username: "user3" }))).toBe("U");
    expect(memberInitial(member({ id: 3, name: "", username: "" }))).toBe("?");
  });
});

describe("memberStateKey", () => {
  it("只给非活跃状态返回 i18n key", () => {
    expect(memberStateKey("active")).toBe("");
    expect(memberStateKey("blocked")).toBe("memberStateBlocked");
    expect(memberStateKey("deactivated")).toBe("memberStateDeactivated");
    expect(memberStateKey("")).toBe("");
  });
});

describe("memberSourceKey", () => {
  it("区分直接成员与继承成员，未知来源返回空串", () => {
    expect(memberSourceKey("direct")).toBe("memberSourceDirect");
    expect(memberSourceKey("inherited")).toBe("memberSourceInherited");
    expect(memberSourceKey("")).toBe("");
    expect(memberSourceKey("other")).toBe("");
  });
});

describe("memberInviteTarget", () => {
  it("含 @ 视为邮箱邀请，否则按用户名添加", () => {
    expect(memberInviteTarget("invitee@example.com")).toEqual({
      kind: "email",
      value: "invitee@example.com",
    });
    expect(memberInviteTarget(" user3 ")).toEqual({ kind: "username", value: "user3" });
    expect(memberInviteTarget("@user3")).toEqual({ kind: "username", value: "user3" });
  });

  it("空输入返回 null", () => {
    expect(memberInviteTarget("")).toBeNull();
    expect(memberInviteTarget("   ")).toBeNull();
    expect(memberInviteTarget("@")).toBeNull();
  });
});

describe("canManageMember", () => {
  const viewer = (id: number, accessLevel: number) => ({ id, accessLevel });

  it("维护者可以管理权限更低的直接成员", () => {
    expect(canManageMember(member({ id: 3, access_level: 30, source: "direct" }), viewer(7, 40))).toBe(
      true,
    );
  });

  it("维护者不能管理权限更高的所有者", () => {
    expect(canManageMember(member({ id: 2, access_level: 50, source: "direct" }), viewer(7, 40))).toBe(
      false,
    );
  });

  it("所有者可以管理维护者", () => {
    expect(canManageMember(member({ id: 4, access_level: 40, source: "direct" }), viewer(7, 50))).toBe(
      true,
    );
  });

  it("开发者及以下不能管理任何成员", () => {
    expect(canManageMember(member({ id: 4, access_level: 20, source: "direct" }), viewer(7, 30))).toBe(
      false,
    );
  });

  it("所有者可以管理同级所有者", () => {
    expect(canManageMember(member({ id: 2, access_level: 50, source: "direct" }), viewer(7, 50))).toBe(
      true,
    );
  });

  it("维护者可以管理同级维护者", () => {
    expect(canManageMember(member({ id: 4, access_level: 40, source: "direct" }), viewer(7, 40))).toBe(
      true,
    );
  });

  it("不能操作自己、继承成员或来源未知的成员", () => {
    expect(canManageMember(member({ id: 7, access_level: 30, source: "direct" }), viewer(7, 40))).toBe(
      false,
    );
    expect(
      canManageMember(member({ id: 3, access_level: 30, source: "inherited" }), viewer(7, 40)),
    ).toBe(false);
    expect(canManageMember(member({ id: 3, access_level: 30 }), viewer(7, 40))).toBe(false);
  });
});

describe("canEditMemberRole", () => {
  const viewer = { id: 7, accessLevel: 40 };
  const target = member({ id: 3, access_level: 30, source: "direct" });

  it("有权限且非只读时才允许改角色", () => {
    expect(canEditMemberRole(target, false, viewer)).toBe(true);
    expect(canEditMemberRole(target, true, viewer)).toBe(false);
    expect(
      canEditMemberRole(member({ id: 2, access_level: 50, source: "direct" }), false, viewer),
    ).toBe(false);
  });
});

describe("invitableRoles", () => {
  const LEVELS = [10, 20, 30, 40, 50];

  it("可分配不高于自己等级的角色（含同级）", () => {
    expect(invitableRoles(40, LEVELS)).toEqual([10, 20, 30, 40]);
    expect(invitableRoles(50, LEVELS)).toEqual([10, 20, 30, 40, 50]);
    expect(invitableRoles(30, LEVELS)).toEqual([10, 20, 30]);
    expect(invitableRoles(0, LEVELS)).toEqual([]);
  });
});

describe("isLastOwner", () => {
  const owner = member({ id: 1, access_level: 50, source: "direct" });
  const maintainer = member({ id: 2, access_level: 40, source: "direct" });

  it("项目只剩一位所有者时不能再动这个所有者", () => {
    expect(isLastOwner(owner, [owner, maintainer])).toBe(true);
  });

  it("还有别的所有者时可以操作", () => {
    const other = member({ id: 3, access_level: 50, source: "inherited" });
    expect(isLastOwner(owner, [owner, other])).toBe(false);
  });

  it("非所有者永远不是最后一个所有者", () => {
    expect(isLastOwner(maintainer, [maintainer])).toBe(false);
  });
});

describe("filterMembers", () => {
  it("按姓名或用户名匹配，忽略大小写", () => {
    expect(ids(filterMembers(team, "USER TWO", 0))).toEqual([2]);
    expect(ids(filterMembers(team, "  user1 ", 0))).toEqual([1]);
  });

  it("按角色等级过滤", () => {
    expect(ids(filterMembers(team, "", 30))).toEqual([3, 5]);
  });

  it("空条件返回全部成员且不改动入参", () => {
    const result = filterMembers(team, "   ", 0);
    expect(ids(result)).toEqual([1, 2, 3, 4, 5]);
    expect(result).not.toBe(team);
  });
});

describe("sortMembers", () => {
  it("角色降序时所有者在前，同级按姓名升序", () => {
    expect(ids(sortMembers(team, "role", "desc"))).toEqual([2, 1, 5, 3, 4]);
  });

  it("角色升序时等级低的在前，同级仍按姓名升序", () => {
    expect(ids(sortMembers(team, "role", "asc"))).toEqual([4, 5, 3, 1, 2]);
  });

  it("姓名排序忽略大小写", () => {
    expect(ids(sortMembers(team, "name", "asc"))).toEqual([5, 4, 1, 3, 2]);
  });

  it("过期时间排序把未设置过期时间的成员恒定放在最后，其余按姓名", () => {
    const expiring: Member[] = [
      member({ id: 1, name: "User Five", expires_at: "2026-12-31T00:00:00Z" }),
      member({ id: 2, name: "User Three", expires_at: "2026-11-15T00:00:00Z" }),
      member({ id: 3, name: "User Two", expires_at: null }),
      member({ id: 4, name: "User One", expires_at: "2026-10-29T00:00:00Z" }),
    ];
    expect(ids(sortMembers(expiring, "expiry", "asc"))).toEqual([4, 2, 1, 3]);
    expect(ids(sortMembers(expiring, "expiry", "desc"))).toEqual([1, 2, 4, 3]);
  });

  it("不修改入参数组", () => {
    const before = ids(team);
    sortMembers(team, "role", "asc");
    expect(ids(team)).toEqual(before);
  });
});
