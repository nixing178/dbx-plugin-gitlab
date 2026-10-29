/**
 * 鼠标侧键 → 导航动作。
 *
 * 侧键编号按 Windows / Chromium 标准：`3` 后退、`4` 前进。
 * 按下与抬起要配对：只收到抬起时也放行（部分宿主不派发 mousedown），
 * 但按下的键和抬起的键不一致时视为误触，直接丢弃。
 */

export type NavAction = "back" | "forward";

export function navActionForButton(button: number): NavAction | null {
  if (button === 3) return "back";
  if (button === 4) return "forward";
  return null;
}

export interface MouseNavTracker {
  down(button: number): void;
  up(button: number): NavAction | null;
}

export function createMouseNavTracker(): MouseNavTracker {
  let pending: NavAction | null = null;
  return {
    down(button) {
      pending = navActionForButton(button);
    },
    up(button) {
      const action = navActionForButton(button);
      const expected = pending;
      pending = null;
      if (!action) return null;
      if (expected && expected !== action) return null;
      return action;
    },
  };
}
