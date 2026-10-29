import { diffStats } from "./diff";
import type { DiffFile } from "../types";

/**
 * MR「变更」页的文件树。
 *
 * GitLab 的变更页是「左栏文件树 + 右栏单文件 diff」，所以这里只负责把扁平的
 * 文件列表折成目录树、按关键词过滤、以及按渲染顺序展开。
 */
export interface DiffTreeNode {
  name: string;
  /** 目录是相对根的前缀，文件是完整路径。 */
  path: string;
  folder: boolean;
  children: DiffTreeNode[];
  file: DiffFile | null;
}

/** 重命名文件以新路径为准。 */
export function fileKey(file: DiffFile): string {
  return file.new_path || file.old_path;
}

function compareNodes(left: DiffTreeNode, right: DiffTreeNode): number {
  if (left.folder !== right.folder) return left.folder ? -1 : 1;
  return left.name.localeCompare(right.name, "en", { sensitivity: "base" });
}

function sortTree(nodes: DiffTreeNode[]): DiffTreeNode[] {
  nodes.sort(compareNodes);
  for (const node of nodes) {
    if (node.folder) sortTree(node.children);
  }
  return nodes;
}

/**
 * 单链目录合并成一行（`src/main/kotlin`），对齐 GitLab 文件树的做法：
 * 36 个文件都在同一个深层目录时，逐层展开会只剩一串缩进。
 */
function compactTree(nodes: DiffTreeNode[]): DiffTreeNode[] {
  return nodes.map((node) => {
    if (!node.folder) return node;

    let tail = node;
    let path = node.path;
    let name = node.name;
    while (tail.children.length === 1 && tail.children[0].folder) {
      const child = tail.children[0];
      name = `${name}/${child.name}`;
      path = child.path;
      tail = child;
    }
    return { ...node, name, path, children: compactTree(tail.children) };
  });
}

export function buildDiffTree(files: DiffFile[]): DiffTreeNode[] {
  const root: DiffTreeNode[] = [];

  for (const file of files) {
    const path = fileKey(file);
    const segments = path.split("/").filter(Boolean);
    if (!segments.length) continue;

    let container = root;
    let prefix = "";
    for (let index = 0; index < segments.length - 1; index += 1) {
      const name = segments[index];
      prefix = prefix ? `${prefix}/${name}` : name;
      let folder = container.find((node) => node.folder && node.path === prefix);
      if (!folder) {
        folder = { name, path: prefix, folder: true, children: [], file: null };
        container.push(folder);
      }
      container = folder.children;
    }

    const name = segments[segments.length - 1];
    container.push({ name, path, folder: false, children: [], file });
  }

  return compactTree(sortTree(root));
}

/** 目录命中时保留整棵子树；文件命中时保留到根的那条路径。 */
export function filterDiffTree(nodes: DiffTreeNode[], query: string): DiffTreeNode[] {
  const keyword = query.trim().toLowerCase();
  if (!keyword) return nodes;

  const result: DiffTreeNode[] = [];
  for (const node of nodes) {
    if (node.path.toLowerCase().includes(keyword)) {
      result.push(node);
      continue;
    }
    if (node.folder) {
      const children = filterDiffTree(node.children, keyword);
      if (children.length) result.push({ ...node, children });
    }
  }
  return result;
}

export function flattenDiffTree(nodes: DiffTreeNode[]): DiffFile[] {
  const files: DiffFile[] = [];
  for (const node of nodes) {
    if (node.folder) {
      files.push(...flattenDiffTree(node.children));
    } else if (node.file) {
      files.push(node.file);
    }
  }
  return files;
}

export function diffTotals(files: DiffFile[]): { additions: number; deletions: number } {
  let additions = 0;
  let deletions = 0;
  for (const file of files) {
    const stats = diffStats(file.diff ?? "");
    additions += stats.additions;
    deletions += stats.deletions;
  }
  return { additions, deletions };
}
