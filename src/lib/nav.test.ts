import { describe, expect, it } from "vitest";
import {
  breadcrumbs,
  projectNavItem,
  projectOf,
  routeKey,
  sidebarSection,
  switchSection,
  type BranchScope,
  type ProjectView,
  type Route,
} from "./nav";

function projectRoute(view: ProjectView): Route {
  return { name: "project", project: "demo/app", view };
}

describe("routeKey", () => {
  it("keeps list filters in the key", () => {
    expect(
      routeKey({
        name: "mrs",
        project: "demo/app",
        scope: "reviewer",
        state: "opened",
        search: "fix",
        orderBy: "updated_at",
        sort: "desc",
      }),
    ).toBe("mrs:demo/app:reviewer:opened:fix:updated_at:desc");
    expect(
      routeKey({ name: "issues", project: "", scope: "todos", state: "opened", search: "" }),
    ).toBe("issues::todos:opened:");
  });

  it("normalises the merge request sort order in the key", () => {
    const plain = routeKey({
      name: "mrs",
      project: "",
      scope: "reviewer",
      state: "opened",
      search: "",
    });
    expect(plain).toBe("mrs::reviewer:opened::updated_at:desc");
    expect(
      routeKey({
        name: "mrs",
        project: "",
        scope: "reviewer",
        state: "opened",
        search: "",
        orderBy: "updated_at",
        sort: "desc",
      }),
    ).toBe(plain);
    expect(
      routeKey({
        name: "mrs",
        project: "",
        scope: "reviewer",
        state: "opened",
        search: "",
        orderBy: "created_at",
        sort: "asc",
      }),
    ).not.toBe(plain);
  });

  it("separates merge request tabs", () => {
    const overview = routeKey({ name: "mr", project: "demo/app", iid: 128, tab: "overview" });
    const changes = routeKey({ name: "mr", project: "demo/app", iid: 128, tab: "changes" });
    expect(overview).toBe("mr:demo/app#128:overview");
    expect(overview).not.toBe(changes);
  });

  it("tracks the job drawer but not pipeline tab state", () => {
    expect(routeKey({ name: "pipeline", project: "demo/app", pipelineId: 9 })).toBe(
      "pipeline:demo/app#9",
    );
    expect(routeKey({ name: "pipeline", project: "demo/app", pipelineId: 9, jobId: 42 })).toBe(
      "pipeline:demo/app#9:42",
    );
  });

  it("separates a commit detail from the commit history", () => {
    const list = routeKey(projectRoute("commits"));
    const detail = routeKey({ name: "commit", project: "demo/app", sha: "4cfa82fc9a1b" });
    expect(detail).toBe("commit:demo/app@4cfa82fc9a1b");
    expect(detail).not.toBe(list);
    expect(routeKey({ name: "commit", project: "demo/app", sha: "a48935cf7a11" })).not.toBe(detail);
  });

  it("covers every route kind", () => {
    const keys = [
      routeKey({ name: "projects" }),
      routeKey(projectRoute("overview")),
      routeKey({ name: "mrs", project: "", scope: "reviewer", state: "opened", search: "" }),
      routeKey({ name: "mr", project: "demo/app", iid: 1, tab: "changes" }),
      routeKey({ name: "pipelines", project: "demo/app", status: "", ref: "main" }),
      routeKey({ name: "pipeline", project: "demo/app", pipelineId: 9 }),
      routeKey({ name: "issues", project: "", scope: "assigned", state: "opened", search: "" }),
      routeKey({ name: "issue", project: "demo/app", iid: 7 }),
      routeKey({ name: "commit", project: "demo/app", sha: "4cfa82fc" }),
      routeKey({ name: "mrNew", project: "demo/app" }),
      routeKey({ name: "branches", project: "demo/app", scope: "overview", search: "" }),
      routeKey({ name: "todos" }),
    ];
    expect(new Set(keys).size).toBe(keys.length);
  });
});

describe("breadcrumbs", () => {
  it("starts at projects for the root route", () => {
    expect(breadcrumbs({ name: "projects" })).toEqual([{ id: "projects" }]);
  });

  it("makes ancestors clickable", () => {
    const crumbs = breadcrumbs({ name: "mr", project: "demo/app", iid: 128, tab: "overview" });
    expect(crumbs.map((crumb) => crumb.id)).toEqual(["projects", "project", "mrs", "mr"]);
    expect(crumbs[1].text).toBe("demo/app");
    expect(crumbs[2].route).toEqual({
      name: "mrs",
      project: "demo/app",
      scope: "project",
      state: "opened",
      search: "",
    });
    expect(crumbs[3].text).toBe("!128");
    expect(crumbs[3].route).toBeUndefined();
  });

  it("drops the project crumb for global lists", () => {
    const crumbs = breadcrumbs({
      name: "issues",
      project: "",
      scope: "assigned",
      state: "opened",
      search: "",
    });
    expect(crumbs.map((crumb) => crumb.id)).toEqual(["projects", "issues"]);
    expect(crumbs[1].route).toBeUndefined();
  });

  it("links pipeline back to the pipeline list of the same project", () => {
    const crumbs = breadcrumbs({ name: "pipeline", project: "demo/app", pipelineId: 9 });
    expect(crumbs.map((crumb) => crumb.id)).toEqual([
      "projects",
      "project",
      "pipelines",
      "pipeline",
    ]);
    expect(crumbs[2].route).toEqual({ name: "pipelines", project: "demo/app", status: "", ref: "" });
    expect(crumbs[3].text).toBe("#9");
  });

  it("links the commit detail back to the commit history of the project", () => {
    const crumbs = breadcrumbs({ name: "commit", project: "demo/app", sha: "4cfa82fc9a1b" });
    expect(crumbs.map((crumb) => crumb.id)).toEqual(["projects", "project", "commits", "commit"]);
    expect(crumbs[1].text).toBe("demo/app");
    expect(crumbs[2].route).toEqual({ name: "project", project: "demo/app", view: "commits" });
    expect(crumbs[3].text).toBe("4cfa82fc");
    expect(crumbs[3].route).toBeUndefined();
  });

  it("appends the commits crumb on the project commits view", () => {
    const crumbs = breadcrumbs(projectRoute("commits"));
    expect(crumbs.map((crumb) => crumb.id)).toEqual(["projects", "project", "commits"]);
    expect(crumbs[1].text).toBe("demo/app");
    expect(crumbs[1].route).toEqual({ name: "project", project: "demo/app", view: "overview" });
    expect(crumbs[2].route).toBeUndefined();
  });

  it("handles todos and issue details", () => {
    expect(breadcrumbs({ name: "todos" }).map((crumb) => crumb.id)).toEqual(["projects", "todos"]);
    const issue = breadcrumbs({ name: "issue", project: "demo/app", iid: 7 });
    expect(issue.map((crumb) => crumb.id)).toEqual(["projects", "project", "issues", "issue"]);
    expect(issue[3].text).toBe("#7");
  });
});

describe("sidebarSection", () => {
  it("maps each route to exactly one global section", () => {
    expect(sidebarSection({ name: "projects" })).toBe("projects");
    expect(sidebarSection(projectRoute("files"))).toBe("projects");
    expect(sidebarSection({ name: "mr", project: "demo/app", iid: 1, tab: "changes" })).toBe("mrs");
    expect(sidebarSection({ name: "pipeline", project: "demo/app", pipelineId: 1 })).toBe(
      "pipelines",
    );
    expect(sidebarSection({ name: "issue", project: "demo/app", iid: 1 })).toBe("issues");
    expect(sidebarSection({ name: "commit", project: "demo/app", sha: "4cfa82fc" })).toBe(
      "projects",
    );
    expect(sidebarSection({ name: "todos" })).toBe("todos");
  });
});

describe("projectNavItem", () => {
  it("highlights the project level nav", () => {
    expect(projectNavItem(projectRoute("overview"))).toBe("code");
    expect(projectNavItem(projectRoute("files"))).toBe("code");
    expect(projectNavItem(projectRoute("search"))).toBe("code");
    expect(projectNavItem(projectRoute("commits"))).toBe("commits");
    expect(projectNavItem(projectRoute("members"))).toBe("members");
    expect(projectNavItem({ name: "mrs", project: "demo/app", scope: "project", state: "opened", search: "" })).toBe("mrs");
    expect(projectNavItem({ name: "mr", project: "demo/app", iid: 1, tab: "overview" })).toBe("mrs");
    expect(projectNavItem({ name: "issues", project: "demo/app", scope: "project", state: "opened", search: "" })).toBe("issues");
    expect(projectNavItem({ name: "issue", project: "demo/app", iid: 1 })).toBe("issues");
    expect(projectNavItem({ name: "pipelines", project: "demo/app", status: "", ref: "" })).toBe("ci");
    expect(projectNavItem({ name: "pipeline", project: "demo/app", pipelineId: 1 })).toBe("ci");
    expect(projectNavItem({ name: "commit", project: "demo/app", sha: "4cfa82fc" })).toBe("code");
  });

  it("returns null outside a project", () => {
    expect(projectNavItem({ name: "projects" })).toBeNull();
    expect(projectNavItem({ name: "todos" })).toBeNull();
    expect(
      projectNavItem({ name: "mrs", project: "", scope: "reviewer", state: "opened", search: "" }),
    ).toBeNull();
  });
});

describe("projectOf", () => {
  it("extracts the project path and defaults to empty", () => {
    expect(projectOf(projectRoute("overview"))).toBe("demo/app");
    expect(projectOf({ name: "projects" })).toBe("");
    expect(projectOf({ name: "todos" })).toBe("");
  });
});

describe("switchSection", () => {
  it("keeps the current project for scoped sections", () => {
    expect(switchSection(projectRoute("files"), "mrs")).toEqual({
      name: "mrs",
      project: "demo/app",
      scope: "project",
      state: "opened",
      search: "",
    });
    expect(switchSection(projectRoute("files"), "issues")).toEqual({
      name: "issues",
      project: "demo/app",
      scope: "project",
      state: "opened",
      search: "",
    });
    expect(switchSection(projectRoute("files"), "pipelines")).toEqual({
      name: "pipelines",
      project: "demo/app",
      status: "",
      ref: "",
    });
    expect(switchSection(projectRoute("files"), "projects")).toEqual({ name: "projects" });
  });

  it("falls back to global scopes without a project", () => {
    expect(switchSection({ name: "todos" }, "mrs")).toEqual({
      name: "mrs",
      project: "",
      scope: "reviewer",
      state: "opened",
      search: "",
    });
    expect(switchSection({ name: "todos" }, "issues")).toEqual({
      name: "issues",
      project: "",
      scope: "todos",
      state: "opened",
      search: "",
    });
    expect(switchSection({ name: "todos" }, "todos")).toEqual({ name: "todos" });
  });

  it("keeps the project when moving between detail pages", () => {
    const mr: Route = { name: "mr", project: "demo/app", iid: 5, tab: "overview" };
    expect(switchSection(mr, "pipelines")).toEqual({
      name: "pipelines",
      project: "demo/app",
      status: "",
      ref: "",
    });
  });
});

describe("merge request creation route", () => {
  const mrNew: Route = { name: "mrNew", project: "demo/project" };

  it("keeps its own route key", () => {
    expect(routeKey(mrNew)).toBe("mrNew:demo/project");
    expect(routeKey({ name: "mrNew", project: "demo/app" })).not.toBe(routeKey(mrNew));
    expect(routeKey(mrNew)).not.toBe(
      routeKey({
        name: "mrs",
        project: "demo/project",
        scope: "project",
        state: "opened",
        search: "",
      }),
    );
  });

  it("hangs under the merge request list with a trailing New crumb", () => {
    const crumbs = breadcrumbs(mrNew);
    expect(crumbs.map((crumb) => crumb.id)).toEqual(["projects", "project", "mrs", "mrNew"]);
    expect(crumbs[2].route).toEqual({
      name: "mrs",
      project: "demo/project",
      scope: "project",
      state: "opened",
      search: "",
    });
    expect(crumbs[3].text).toBeUndefined();
    expect(crumbs[3].route).toBeUndefined();
  });

  it("highlights the merge request entry", () => {
    expect(sidebarSection(mrNew)).toBe("mrs");
    expect(projectNavItem(mrNew)).toBe("mrs");
    expect(projectOf(mrNew)).toBe("demo/project");
  });

  it("keeps the project context when switching away", () => {
    expect(switchSection(mrNew, "issues")).toEqual({
      name: "issues",
      project: "demo/project",
      scope: "project",
      state: "opened",
      search: "",
    });
  });
});

describe("branch routes", () => {
  const branches = (scope: BranchScope, search = ""): Route => ({
    name: "branches",
    project: "demo/app",
    scope,
    search,
  });

  it("keeps the tab and the name filter in the route key", () => {
    expect(routeKey(branches("active"))).toBe("branches:demo/app:active:");
    expect(routeKey(branches("active"))).not.toBe(routeKey(branches("stale")));
    expect(routeKey(branches("overview", "feat"))).not.toBe(routeKey(branches("overview")));
  });

  it("hangs under the project crumb", () => {
    const crumbs = breadcrumbs(branches("overview"));
    expect(crumbs.map((crumb) => crumb.id)).toEqual(["projects", "project", "branches"]);
    expect(crumbs[1].route).toEqual({ name: "project", project: "demo/app", view: "overview" });
    expect(crumbs[2].route).toBeUndefined();
  });

  it("highlights the project level branches entry", () => {
    expect(projectNavItem(branches("all"))).toBe("branches");
    expect(sidebarSection(branches("all"))).toBe("projects");
    expect(projectOf(branches("all"))).toBe("demo/app");
  });
});

describe("branch creation route", () => {
  const branchNew: Route = { name: "branchNew", project: "demo/app" };

  it("keeps its own route key", () => {
    expect(routeKey(branchNew)).toBe("branchNew:demo/app");
    expect(routeKey({ name: "branchNew", project: "demo/other" })).not.toBe(routeKey(branchNew));
    expect(routeKey(branchNew)).not.toBe(
      routeKey({ name: "branches", project: "demo/app", scope: "overview", search: "" }),
    );
  });

  it("hangs under the branch list with a trailing New crumb", () => {
    const crumbs = breadcrumbs(branchNew);
    expect(crumbs.map((crumb) => crumb.id)).toEqual([
      "projects",
      "project",
      "branches",
      "branchNew",
    ]);
    expect(crumbs[2].route).toEqual({
      name: "branches",
      project: "demo/app",
      scope: "overview",
      search: "",
    });
    expect(crumbs[3].text).toBeUndefined();
    expect(crumbs[3].route).toBeUndefined();
  });

  it("highlights the project level branches entry", () => {
    expect(projectNavItem(branchNew)).toBe("branches");
    expect(sidebarSection(branchNew)).toBe("projects");
    expect(projectOf(branchNew)).toBe("demo/app");
  });

  it("keeps the project context when switching away", () => {
    expect(switchSection(branchNew, "issues")).toEqual({
      name: "issues",
      project: "demo/app",
      scope: "project",
      state: "opened",
      search: "",
    });
  });
});

describe("branch scoped project route", () => {
  const projectAt = (ref?: string): Route => ({
    name: "project",
    project: "demo/app",
    view: "overview",
    ...(ref ? { ref } : {}),
  });

  it("keeps the branch in the route key", () => {
    const plain = routeKey(projectAt());
    expect(routeKey(projectAt("feature/x"))).not.toBe(plain);
    expect(routeKey(projectAt("feature/x"))).not.toBe(routeKey(projectAt("feature/y")));
  });

  it("hangs under the branch list with the branch name", () => {
    const crumbs = breadcrumbs(projectAt("feature/x"));
    expect(crumbs.map((crumb) => crumb.id)).toEqual([
      "projects",
      "project",
      "branches",
      "branch",
    ]);
    expect(crumbs[2].route).toEqual({
      name: "branches",
      project: "demo/app",
      scope: "overview",
      search: "",
    });
    expect(crumbs[3].text).toBe("feature/x");
    expect(crumbs[3].route).toBeUndefined();
  });

  it("leaves the plain project crumb unchanged without a ref", () => {
    expect(breadcrumbs(projectAt()).map((crumb) => crumb.id)).toEqual(["projects", "project"]);
  });

  it("keeps the project context when switching away", () => {
    expect(switchSection(projectAt("feature/x"), "mrs")).toEqual({
      name: "mrs",
      project: "demo/app",
      scope: "project",
      state: "opened",
      search: "",
    });
  });
});
