import { describe, expect, it } from "vitest";
import {
  DEFAULT_MR_ORDER,
  DEFAULT_MR_SORT,
  MR_ORDER_BYS,
  MR_STATES,
  UNKNOWN_COUNT,
  mrCountLabel,
  mrCountsQuery,
  mrEmptyCopy,
  mrListState,
  mrQuery,
  mrSort,
  mrTabs,
  type MrFilterInput,
} from "./mrFilters";

const projectFilter: MrFilterInput = {
  project: "demo/project",
  scope: "project",
  state: "opened",
  search: "",
  orderBy: "",
  sort: "",
};

const globalFilter: MrFilterInput = {
  project: "",
  scope: "reviewer",
  state: "opened",
  search: "",
  orderBy: "",
  sort: "",
};

describe("mrTabs", () => {
  it("keeps the GitLab tab order and counts", () => {
    expect(mrTabs({ opened: 0, merged: 214, closed: 20, all: 234 })).toEqual([
      { state: "opened", count: 0 },
      { state: "merged", count: 214 },
      { state: "closed", count: 20 },
      { state: "all", count: 234 },
    ]);
  });

  it("marks every tab unknown when counts are unavailable", () => {
    const tabs = mrTabs(null);
    expect(tabs.map((tab) => tab.state)).toEqual(MR_STATES);
    expect(tabs.every((tab) => tab.count === UNKNOWN_COUNT)).toBe(true);
  });
});

describe("mrCountLabel", () => {
  it("prints real counts, including zero", () => {
    expect(mrCountLabel(0)).toBe("0");
    expect(mrCountLabel(234)).toBe("234");
  });

  it("hides unknown counts instead of pretending they are zero", () => {
    expect(mrCountLabel(UNKNOWN_COUNT)).toBe("");
  });
});

describe("mrListState", () => {
  it("accepts every GitLab state", () => {
    expect(mrListState("opened")).toBe("opened");
    expect(mrListState("merged")).toBe("merged");
    expect(mrListState("closed")).toBe("closed");
    expect(mrListState("all")).toBe("all");
  });

  it("falls back to opened for dirty routes", () => {
    expect(mrListState("")).toBe("opened");
    expect(mrListState("locked")).toBe("opened");
  });
});

describe("mrSort", () => {
  it("defaults to updated_at / desc like the GitLab page", () => {
    expect(mrSort()).toEqual({ orderBy: DEFAULT_MR_ORDER, sort: DEFAULT_MR_SORT });
    expect(DEFAULT_MR_ORDER).toBe("updated_at");
    expect(DEFAULT_MR_SORT).toBe("desc");
  });

  it("passes legal values through", () => {
    expect(mrSort("created_at", "asc")).toEqual({ orderBy: "created_at", sort: "asc" });
  });

  it("falls back to defaults on unknown values", () => {
    expect(mrSort("milestone_due", "sideways")).toEqual({
      orderBy: DEFAULT_MR_ORDER,
      sort: DEFAULT_MR_SORT,
    });
  });

  it("exposes the supported order fields", () => {
    expect(MR_ORDER_BYS).toEqual(["updated_at", "created_at"]);
  });
});

describe("mrQuery", () => {
  it("drops scope for project views and fills the default sort", () => {
    expect(mrQuery(projectFilter)).toEqual({
      projectId: "demo/project",
      scope: "",
      state: "opened",
      search: "",
      orderBy: "updated_at",
      sort: "desc",
    });
  });

  it("keeps scope for global views", () => {
    expect(mrQuery(globalFilter)).toEqual({
      projectId: "",
      scope: "reviewer",
      state: "opened",
      search: "",
      orderBy: "updated_at",
      sort: "desc",
    });
  });

  it("omits the state filter for the All tab", () => {
    expect(mrQuery({ ...projectFilter, state: "all" }).state).toBe("");
  });

  it("normalises dirty state values", () => {
    expect(mrQuery({ ...projectFilter, state: "bogus" }).state).toBe("opened");
  });

  it("passes the chosen sort through", () => {
    const query = mrQuery({ ...projectFilter, orderBy: "created_at", sort: "asc" });
    expect(query.orderBy).toBe("created_at");
    expect(query.sort).toBe("asc");
  });

  it("keeps the search term", () => {
    expect(mrQuery({ ...projectFilter, search: " 示例 " }).search).toBe("示例");
  });
});

describe("mrCountsQuery", () => {
  it("reuses project and scope without paging or state", () => {
    expect(mrCountsQuery(projectFilter)).toEqual({
      projectId: "demo/project",
      scope: "",
      search: "",
    });
    expect(mrCountsQuery({ ...globalFilter, search: "示例" })).toEqual({
      projectId: "",
      scope: "reviewer",
      search: "示例",
    });
  });
});

describe("mrEmptyCopy", () => {
  it("explains each state in GitLab wording", () => {
    expect(mrEmptyCopy("opened", "")).toEqual({
      titleKey: "mrEmptyOpenedTitle",
      hintKey: "mrEmptyOpenedHint",
    });
    expect(mrEmptyCopy("merged", "")).toEqual({
      titleKey: "mrEmptyMergedTitle",
      hintKey: "mrEmptyMergedHint",
    });
    expect(mrEmptyCopy("closed", "")).toEqual({
      titleKey: "mrEmptyClosedTitle",
      hintKey: "mrEmptyClosedHint",
    });
    expect(mrEmptyCopy("all", "")).toEqual({
      titleKey: "mrEmptyAllTitle",
      hintKey: "mrEmptyAllHint",
    });
  });

  it("prefers the search wording whenever a query is present", () => {
    expect(mrEmptyCopy("opened", "示例")).toEqual({
      titleKey: "mrEmptySearchTitle",
      hintKey: "mrEmptySearchHint",
    });
    expect(mrEmptyCopy("merged", "  ")).toEqual({
      titleKey: "mrEmptyMergedTitle",
      hintKey: "mrEmptyMergedHint",
    });
  });
});
