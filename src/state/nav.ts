import { computed, reactive } from "vue";
import {
  breadcrumbs,
  projectNavItem,
  projectOf,
  routeKey,
  sidebarSection,
  switchSection,
  type Route,
  type Section,
} from "../lib/nav";
import {
  canGoBack as hasPrevious,
  canGoForward as hasNext,
  createHistory,
  currentOf,
  DEFAULT_HISTORY_LIMIT,
  pushEntry,
  replaceEntry,
  step,
  type NavHistory,
} from "../lib/navHistory";

/**
 * 全局唯一的页面状态。Route 是可序列化的纯数据，
 * 组件只调用 `go()` / `replace()`，不再各自维护「当前选中」副本。
 *
 * 历史是一条 `entries` 加指针 `index`：`go()` 压新页并作废前进分支，
 * `back()` / `forward()` 只移动指针；鼠标侧键与顶栏箭头共用这一份状态。
 */
const INITIAL_ROUTE: Route = { name: "projects" };
const MAX_HISTORY = DEFAULT_HISTORY_LIMIT;

const history = reactive<NavHistory<Route>>(createHistory(INITIAL_ROUTE));

export const route = computed<Route>(() => currentOf(history));

export const crumbs = computed(() => breadcrumbs(route.value));
export const activeSection = computed(() => sidebarSection(route.value));
export const activeProjectNav = computed(() => projectNavItem(route.value));
export const currentProjectPath = computed(() => projectOf(route.value));
export const currentKey = computed(() => routeKey(route.value));

/** 顶栏箭头与鼠标侧键共用：还有没有上一页 / 下一页。 */
export const canGoBack = computed(() => hasPrevious(history));
export const canGoForward = computed(() => hasNext(history));

function apply(next: NavHistory<Route> | null): void {
  if (!next) return;
  history.entries = [...next.entries];
  history.index = next.index;
}

/** 换页：压栈，供详情页返回。同键直接忽略，避免重复点同一入口刷栈。 */
export function go(next: Route): void {
  if (routeKey(next) === routeKey(route.value)) return;
  apply(pushEntry(history, next, MAX_HISTORY));
}

/** 同页参数变化（筛选、tab、抽屉）：不压栈。 */
export function replace(next: Route): void {
  apply(replaceEntry(history, next));
}

export function back(): void {
  apply(step(history, -1));
}

export function forward(): void {
  apply(step(history, 1));
}

/** 全局左栏点击：保留当前项目上下文。 */
export function goSection(section: Section): void {
  go(switchSection(route.value, section));
}
