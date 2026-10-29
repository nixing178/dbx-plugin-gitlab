import { describe, expect, it } from "vitest";
import { createMouseNavTracker, navActionForButton } from "./mouseNav";

describe("navActionForButton", () => {
  it("maps side buttons to navigation", () => {
    expect(navActionForButton(3)).toBe("back");
    expect(navActionForButton(4)).toBe("forward");
  });

  it("ignores primary, middle, secondary and bogus buttons", () => {
    expect(navActionForButton(0)).toBeNull();
    expect(navActionForButton(1)).toBeNull();
    expect(navActionForButton(2)).toBeNull();
    expect(navActionForButton(7)).toBeNull();
    expect(navActionForButton(-1)).toBeNull();
    expect(navActionForButton(Number.NaN)).toBeNull();
  });
});

describe("createMouseNavTracker", () => {
  it("fires when the same side button goes down and up", () => {
    const tracker = createMouseNavTracker();
    tracker.down(3);
    expect(tracker.up(3)).toBe("back");
    tracker.down(4);
    expect(tracker.up(4)).toBe("forward");
  });

  it("ignores a mismatched button release", () => {
    const tracker = createMouseNavTracker();
    tracker.down(3);
    expect(tracker.up(4)).toBeNull();
    expect(tracker.up(3)).toBe("back");
  });

  it("still fires when only the release reaches the page", () => {
    const tracker = createMouseNavTracker();
    expect(tracker.up(3)).toBe("back");
    expect(tracker.up(3)).toBe("back");
  });

  it("keeps a pending side button through unrelated clicks", () => {
    const tracker = createMouseNavTracker();
    tracker.down(1);
    tracker.down(3);
    expect(tracker.up(1)).toBeNull();
    expect(tracker.up(3)).toBe("back");
  });
});
