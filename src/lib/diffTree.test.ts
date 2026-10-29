import { describe, expect, it } from "vitest";
import { buildDiffTree, diffTotals, filterDiffTree, flattenDiffTree } from "./diffTree";
import type { DiffFile } from "../types";

function file(path: string, diff = "@@ -1 +1 @@\n-a\n+b"): DiffFile {
  return {
    old_path: path,
    new_path: path,
    a_mode: "100644",
    b_mode: "100644",
    new_file: false,
    renamed_file: false,
    deleted_file: false,
    too_large: false,
    collapsed: false,
    generated_file: false,
    diff,
  };
}

describe("buildDiffTree", () => {
  it("把同目录的文件收进一个目录节点", () => {
    const tree = buildDiffTree([file("src/a.kt"), file("src/b.kt")]);

    expect(tree).toHaveLength(1);
    expect(tree[0]).toMatchObject({ name: "src", path: "src", folder: true });
    expect(tree[0].children.map((node) => node.name)).toEqual(["a.kt", "b.kt"]);
    expect(tree[0].children[0].file?.new_path).toBe("src/a.kt");
  });

  it("单链目录合并成一行（对齐 GitLab 文件树）", () => {
    const tree = buildDiffTree([file("src/main/kotlin/App.kt")]);

    expect(tree).toHaveLength(1);
    expect(tree[0]).toMatchObject({
      name: "src/main/kotlin",
      path: "src/main/kotlin",
      folder: true,
    });
    expect(tree[0].children[0]).toMatchObject({ name: "App.kt", folder: false });
  });

  it("有分叉时不合并目录", () => {
    const tree = buildDiffTree([file("src/main/App.kt"), file("src/test/AppTest.kt")]);

    expect(tree[0].name).toBe("src");
    expect(tree[0].children.map((node) => node.name)).toEqual(["main", "test"]);
  });

  it("目录排在文件前面，同级按名称升序", () => {
    const tree = buildDiffTree([file("root.kt"), file("zeta/x.kt"), file("alpha/y.kt")]);

    expect(tree.map((node) => node.name)).toEqual(["alpha", "zeta", "root.kt"]);
  });

  it("根目录文件直接挂在顶层", () => {
    const tree = buildDiffTree([file("README.md"), file("build.gradle.kts")]);

    expect(tree.every((node) => node.folder === false)).toBe(true);
    expect(tree.map((node) => node.name)).toEqual(["build.gradle.kts", "README.md"]);
  });

  it("重命名文件用 new_path 建树", () => {
    const renamed = { ...file("old.kt"), new_path: "new.kt" };
    const tree = buildDiffTree([renamed]);

    expect(tree[0].name).toBe("new.kt");
    expect(tree[0].file?.old_path).toBe("old.kt");
  });
});

describe("filterDiffTree", () => {
  const tree = buildDiffTree([
    file("src/service/UserService.kt"),
    file("src/config/AppConfig.kt"),
    file("README.md"),
  ]);

  it("关键词为空时原样返回", () => {
    expect(filterDiffTree(tree, "   ")).toBe(tree);
  });

  it("命中文件名时保留整条父目录链", () => {
    const filtered = filterDiffTree(tree, "userservice");

    expect(filtered).toHaveLength(1);
    expect(filtered[0].name).toBe("src");
    expect(filtered[0].children[0].name).toBe("service");
    expect(filtered[0].children[0].children[0].name).toBe("UserService.kt");
  });

  it("命中目录名时保留该目录下所有文件", () => {
    const filtered = filterDiffTree(tree, "config");

    expect(filtered).toHaveLength(1);
    expect(filtered[0].children[0].children.map((node) => node.name)).toEqual(["AppConfig.kt"]);
  });

  it("没有匹配时返回空数组", () => {
    expect(filterDiffTree(tree, "nothing-here")).toEqual([]);
  });
});

describe("flattenDiffTree", () => {
  it("按渲染顺序展开，目录在前", () => {
    const tree = buildDiffTree([file("root.kt"), file("src/a.kt"), file("src/b.kt")]);
    const files = flattenDiffTree(tree).map((item) => item.new_path);

    expect(files).toEqual(["src/a.kt", "src/b.kt", "root.kt"]);
  });
});

describe("diffTotals", () => {
  it("累加所有文件的增删行数", () => {
    const files = [
      file("a.kt", "@@ -1,2 +1,3 @@\n-a\n-b\n+x\n+y\n+z"),
      file("b.kt", "@@ -1 +1 @@\n-old\n+new"),
    ];

    expect(diffTotals(files)).toEqual({ additions: 4, deletions: 3 });
  });

  it("忽略没有 diff 正文的文件", () => {
    expect(diffTotals([file("empty.kt", "")])).toEqual({ additions: 0, deletions: 0 });
  });
});
