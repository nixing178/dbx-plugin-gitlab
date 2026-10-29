import { describe, expect, it } from "vitest";
import {
  canGoBack,
  canGoForward,
  createHistory,
  currentOf,
  pushEntry,
  replaceEntry,
  step,
} from "./navHistory";

describe("navHistory", () => {
  it("starts with one entry and no navigation available", () => {
    const history = createHistory("home");
    expect(currentOf(history)).toBe("home");
    expect(canGoBack(history)).toBe(false);
    expect(canGoForward(history)).toBe(false);
  });

  it("pushes entries and walks back", () => {
    let history = createHistory("home");
    history = pushEntry(history, "list");
    history = pushEntry(history, "detail");
    expect(currentOf(history)).toBe("detail");
    expect(canGoBack(history)).toBe(true);

    const previous = step(history, -1);
    expect(previous).not.toBeNull();
    history = previous!;
    expect(currentOf(history)).toBe("list");
    expect(canGoForward(history)).toBe(true);
  });

  it("walks forward again after going back", () => {
    let history = createHistory("home");
    history = pushEntry(history, "detail");
    history = step(history, -1)!;
    history = step(history, 1)!;
    expect(currentOf(history)).toBe("detail");
    expect(canGoForward(history)).toBe(false);
  });

  it("drops the forward branch after a new push", () => {
    let history = createHistory("home");
    history = pushEntry(history, "a");
    history = pushEntry(history, "b");
    history = step(history, -1)!;
    history = pushEntry(history, "c");
    expect(currentOf(history)).toBe("c");
    expect(canGoForward(history)).toBe(false);
    expect(step(history, 1)).toBeNull();
    expect(history.entries).toEqual(["home", "a", "c"]);
  });

  it("replaces the current entry without touching the position", () => {
    let history = createHistory("home");
    history = pushEntry(history, "a");
    history = replaceEntry(history, "a2");
    expect(currentOf(history)).toBe("a2");
    expect(history.entries).toHaveLength(2);
    expect(canGoBack(history)).toBe(true);
    history = step(history, -1)!;
    expect(currentOf(history)).toBe("home");
    history = step(history, 1)!;
    expect(currentOf(history)).toBe("a2");
  });

  it("stops at both ends without moving", () => {
    let history = createHistory("home");
    expect(step(history, -1)).toBeNull();
    history = pushEntry(history, "a");
    expect(step(history, 1)).toBeNull();
    expect(currentOf(history)).toBe("a");
  });

  it("drops the oldest entry beyond the limit", () => {
    let history = createHistory("r0");
    for (let i = 1; i <= 4; i += 1) {
      history = pushEntry(history, `r${i}`, 3);
    }
    expect(history.entries).toEqual(["r2", "r3", "r4"]);
    expect(currentOf(history)).toBe("r4");
    history = step(history, -1)!;
    history = step(history, -1)!;
    expect(currentOf(history)).toBe("r2");
    expect(step(history, -1)).toBeNull();
  });

  it("keeps at least the current entry when the limit is degenerate", () => {
    let history = createHistory("home");
    history = pushEntry(history, "a", 0);
    expect(history.entries).toEqual(["a"]);
    expect(currentOf(history)).toBe("a");
    expect(canGoBack(history)).toBe(false);
    history = replaceEntry(history, "a2");
    expect(currentOf(history)).toBe("a2");
  });
});
