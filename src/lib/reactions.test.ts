import { describe, expect, it } from "vitest";
import { COMMON_REACTIONS, glyphOf, reactionSummary } from "./reactions";
import type { AwardEmoji, GitLabUser } from "../types";

function user(id: number, username: string): GitLabUser {
  return { id, username, name: username, avatar_url: "", web_url: "" };
}

function award(id: number, name: string, owner: GitLabUser | null): AwardEmoji {
  return { id, name, user: owner, awardable_id: 128, awardable_type: "MergeRequest" };
}

describe("reactionSummary", () => {
  it("没有反应时也固定给出常用项，计数为 0", () => {
    const summary = reactionSummary([], 1);

    expect(summary.map((item) => item.name)).toEqual([...COMMON_REACTIONS]);
    expect(summary.every((item) => item.count === 0 && !item.mine)).toBe(true);
  });

  it("按 name 聚合计数，并标出自己那条的 id", () => {
    const summary = reactionSummary(
      [award(11, "thumbsup", user(2, "user6")), award(12, "thumbsup", user(1, "me"))],
      1,
    );
    const thumbsup = summary.find((item) => item.name === "thumbsup");

    expect(thumbsup).toMatchObject({ count: 2, mine: true, myId: 12 });
  });

  it("只用别人的反应时不标记 mine", () => {
    const summary = reactionSummary([award(11, "tada", user(2, "user6"))], 1);
    const tada = summary.find((item) => item.name === "tada");

    expect(tada).toMatchObject({ count: 1, mine: false, myId: 0 });
  });

  it("非常用反应追加在常用项之后，未知名称用 :name: 兜底", () => {
    const summary = reactionSummary([award(21, "rocket", user(2, "user6")), award(22, "party_parrot", null)], 1);

    expect(summary.map((item) => item.name)).toEqual([
      ...COMMON_REACTIONS,
      "rocket",
      "party_parrot",
    ]);
    expect(summary.find((item) => item.name === "rocket")?.glyph).toBe("🚀");
    expect(summary.find((item) => item.name === "party_parrot")?.glyph).toBe(":party_parrot:");
  });

  it("meId 为 0 时不会把 id 0 的反应当自己", () => {
    const summary = reactionSummary([award(31, "eyes", { ...user(0, "ghost") })], 0);

    expect(summary.find((item) => item.name === "eyes")?.mine).toBe(false);
  });

  it("glyphOf 覆盖常用别名", () => {
    expect(glyphOf("thumbsdown")).toBe("👎");
    expect(glyphOf("100")).toBe("💯");
  });
});
