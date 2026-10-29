import type { AwardEmoji } from "../types";

/**
 * 合并请求的「表情回应」聚合。
 *
 * 常用反应固定展示（即使 0 个），其余只在有人用过时出现；
 * `mine` 决定点击是追加还是取消，`myId` 是取消时要删除的那一条。
 */
export const COMMON_REACTIONS = ["thumbsup", "thumbsdown", "tada"] as const;

const GLYPHS: Record<string, string> = {
  thumbsup: "👍",
  thumbsdown: "👎",
  tada: "🎉",
  smile: "😄",
  heart: "❤️",
  rocket: "🚀",
  eyes: "👀",
  confused: "😕",
  wave: "👋",
  "100": "💯",
};

export interface ReactionSummary {
  name: string;
  glyph: string;
  count: number;
  mine: boolean;
  myId: number;
}

/** GitLab 的 name 是英文别名；没有映射时退回 `:name:`，不要吞掉未知反应。 */
export function glyphOf(name: string): string {
  return GLYPHS[name] ?? `:${name}:`;
}

export function reactionSummary(awards: AwardEmoji[], meId = 0): ReactionSummary[] {
  const summary = new Map<string, ReactionSummary>();

  for (const name of COMMON_REACTIONS) {
    summary.set(name, { name, glyph: glyphOf(name), count: 0, mine: false, myId: 0 });
  }

  for (const award of awards) {
    if (!award?.name) continue;
    const entry =
      summary.get(award.name) ??
      { name: award.name, glyph: glyphOf(award.name), count: 0, mine: false, myId: 0 };
    entry.count += 1;
    if (meId > 0 && award.user?.id === meId) {
      entry.mine = true;
      entry.myId = award.id;
    }
    summary.set(award.name, entry);
  }

  return [...summary.values()].filter(
    (item) => item.count > 0 || (COMMON_REACTIONS as readonly string[]).includes(item.name),
  );
}
