import { createMouseNavTracker } from "../lib/mouseNav";
import { back, forward } from "./nav";

/**
 * 鼠标前进 / 后退键驱动页面导航。
 *
 * 插件跑在 iframe 里，浏览器历史不属于插件，侧键的默认导航不会落到内部页面，
 * 所以在 window 的捕获阶段直接接事件：按下记键，抬起同键才换页。
 * 到边界时 `back()` / `forward()` 是空操作，不会报错。
 */
export function installMouseNavigation(target: Window = window): () => void {
  const tracker = createMouseNavTracker();

  const onDown = (event: MouseEvent): void => {
    tracker.down(event.button);
  };

  const onUp = (event: MouseEvent): void => {
    if (event.defaultPrevented) return;
    const action = tracker.up(event.button);
    if (action === "back") back();
    else if (action === "forward") forward();
  };

  target.addEventListener("mousedown", onDown, true);
  target.addEventListener("mouseup", onUp, true);

  return () => {
    target.removeEventListener("mousedown", onDown, true);
    target.removeEventListener("mouseup", onUp, true);
  };
}
