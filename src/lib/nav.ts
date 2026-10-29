/**
 * 导航模型（纯函数）。
 *
 * 插件是单文件构建，不引 vue-router；页面状态用可序列化的 Route 描述，
 * 由 `state/nav.ts` 持有。这里只负责「路由 → 面包屑 / 侧栏高亮 / key」的推导，
 * 方便单测覆盖跳转规则。
 */

export type MrScope = "project" | "reviewer" | "created_by_me" | "assigned_to_me";
export type IssueScope = "todos" | "assigned" | "created" | "project";
export type ProjectView = "overview" | "files" | "commits" | "search" | "members";
export type MrTab = "overview" | "commits" | "changes" | "pipelines";

/** 全局左栏的一级分区。 */
export type Section = "projects" | "mrs" | "pipelines" | "issues" | "todos";

/** 项目内二级导航。 */
export type ProjectNav = "code" | "issues" | "mrs" | "ci";

export type CrumbId =
  | "projects"
  | "project"
  | "commits"
  | "commit"
  | "mrs"
  | "mr"
  | "mrNew"
  | "pipelines"
  | "pipeline"
  | "issues"
  | "issue"
  | "todos";

export interface Crumb {
  id: CrumbId;
  /** 动态文本（项目路径、!128、#9）；缺省时由 i18n 按 id 取文案。 */
  text?: string;
  /** 有值表示可点击回跳；当前页不带 route。 */
  route?: Route;
}

/** `project` 为空串表示「全局视图」，不是「未选择」，避免可选字段到处判空。 */
export type Route =
  | { name: "projects" }
  | { name: "project"; project: string; view: ProjectView }
  | { name: "commit"; project: string; sha: string }
  | {
      name: "mrs";
      project: string;
      scope: MrScope;
      state: string;
      search: string;
      /** 排序字段与方向；缺省由 lib/mrFilters 归一化为 updated_at / desc。 */
      orderBy?: string;
      sort?: string;
    }
  | { name: "mr"; project: string; iid: number; tab: MrTab }
  | { name: "mrNew"; project: string }
  | { name: "pipelines"; project: string; status: string; ref: string }
  | { name: "pipeline"; project: string; pipelineId: number; jobId?: number }
  | { name: "issues"; project: string; scope: IssueScope; state: string; search: string }
  | { name: "issue"; project: string; iid: number }
  | { name: "todos" };

export function projectOf(route: Route): string {
  return "project" in route ? route.project : "";
}

/** 稳定的路由键：同键不重建滚动位置，异键才视为换页。 */
export function routeKey(route: Route): string {
  switch (route.name) {
    case "projects":
      return "projects";
    case "todos":
      return "todos";
    case "project":
      return `project:${route.project}:${route.view}`;
    case "commit":
      return `commit:${route.project}@${route.sha}`;
    case "mrs":
      // 默认排序与 lib/mrFilters 的 DEFAULT_MR_ORDER / DEFAULT_MR_SORT 保持一致，
      // 这样「没传排序」和「显式传默认排序」共享同一条历史记录。
      return `mrs:${route.project}:${route.scope}:${route.state}:${route.search}:${
        route.orderBy ?? "updated_at"
      }:${route.sort ?? "desc"}`;
    case "mr":
      return `mr:${route.project}#${route.iid}:${route.tab}`;
    case "mrNew":
      return `mrNew:${route.project}`;
    case "pipelines":
      return `pipelines:${route.project}:${route.status}:${route.ref}`;
    case "pipeline":
      return `pipeline:${route.project}#${route.pipelineId}${route.jobId ? `:${route.jobId}` : ""}`;
    case "issues":
      return `issues:${route.project}:${route.scope}:${route.state}:${route.search}`;
    case "issue":
      return `issue:${route.project}#${route.iid}`;
  }
}

export function breadcrumbs(route: Route): Crumb[] {
  const root: Crumb = { id: "projects", route: { name: "projects" } };
  const projectCrumb = (project: string): Crumb => ({
    id: "project",
    text: project,
    route: { name: "project", project, view: "overview" },
  });
  const mrListCrumb = (project: string): Crumb => ({
    id: "mrs",
    route: { name: "mrs", project, scope: "project", state: "opened", search: "" },
  });
  const pipelineListCrumb = (project: string): Crumb => ({
    id: "pipelines",
    route: { name: "pipelines", project, status: "", ref: "" },
  });
  const issueListCrumb = (project: string): Crumb => ({
    id: "issues",
    route: { name: "issues", project, scope: "project", state: "opened", search: "" },
  });

  switch (route.name) {
    case "projects":
      return [{ id: "projects" }];
    case "todos":
      return [root, { id: "todos" }];
    case "project":
      return [root, { id: "project", text: route.project }];
    case "commit":
      return [
        root,
        projectCrumb(route.project),
        { id: "commits", route: { name: "project", project: route.project, view: "commits" } },
        { id: "commit", text: route.sha.slice(0, 8) },
      ];
    case "mrs":
      return route.project ? [root, projectCrumb(route.project), { id: "mrs" }] : [root, { id: "mrs" }];
    case "mr":
      return [
        root,
        projectCrumb(route.project),
        mrListCrumb(route.project),
        { id: "mr", text: `!${route.iid}` },
      ];
    case "mrNew":
      return [root, projectCrumb(route.project), mrListCrumb(route.project), { id: "mrNew" }];
    case "pipelines":
      return route.project
        ? [root, projectCrumb(route.project), { id: "pipelines" }]
        : [root, { id: "pipelines" }];
    case "pipeline":
      return [
        root,
        projectCrumb(route.project),
        pipelineListCrumb(route.project),
        { id: "pipeline", text: `#${route.pipelineId}` },
      ];
    case "issues":
      return route.project
        ? [root, projectCrumb(route.project), { id: "issues" }]
        : [root, { id: "issues" }];
    case "issue":
      return [
        root,
        projectCrumb(route.project),
        issueListCrumb(route.project),
        { id: "issue", text: `#${route.iid}` },
      ];
  }
}

export function sidebarSection(route: Route): Section {
  switch (route.name) {
    case "projects":
    case "project":
    case "commit":
      return "projects";
    case "mrs":
    case "mr":
    case "mrNew":
      return "mrs";
    case "pipelines":
    case "pipeline":
      return "pipelines";
    case "issues":
    case "issue":
      return "issues";
    case "todos":
      return "todos";
  }
}

export function projectNavItem(route: Route): ProjectNav | null {
  if (!projectOf(route)) return null;
  switch (route.name) {
    case "project":
    case "commit":
      return "code";
    case "mrs":
    case "mr":
    case "mrNew":
      return "mrs";
    case "issues":
    case "issue":
      return "issues";
    case "pipelines":
    case "pipeline":
      return "ci";
    default:
      return null;
  }
}

/** 点全局左栏：保留当前项目上下文，列表筛选回到默认值。 */
export function switchSection(route: Route, section: Section): Route {
  const project = projectOf(route);
  switch (section) {
    case "projects":
      return { name: "projects" };
    case "todos":
      return { name: "todos" };
    case "mrs":
      return {
        name: "mrs",
        project,
        scope: project ? "project" : "reviewer",
        state: "opened",
        search: "",
      };
    case "issues":
      return {
        name: "issues",
        project,
        scope: project ? "project" : "todos",
        state: "opened",
        search: "",
      };
    case "pipelines":
      return { name: "pipelines", project, status: "", ref: "" };
  }
}
