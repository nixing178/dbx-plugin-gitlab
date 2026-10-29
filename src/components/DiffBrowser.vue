<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { t } from "../i18n";
import { diffStats } from "../lib/diff";
import { diffMissingFiles } from "../lib/diffPaging";
import {
  buildDiffTree,
  diffTotals,
  fileKey,
  filterDiffTree,
  flattenDiffTree,
  type DiffTreeNode,
} from "../lib/diffTree";
import { loadViewed, saveViewed, viewedKey } from "../lib/viewed";
import DiffFile from "./DiffFile.vue";
import Icon from "./Icon.vue";
import type { CommentPosition, DiffFile as DiffFileModel, DiffRefs } from "../types";

/**
 * GitLab 式「变更」页：左栏文件树 + 右栏单文件 diff。
 *
 * 与纵向堆叠相比，这里一次只渲染一个文件，36 个文件的大 MR 也不会把 DOM 撑爆。
 */
const props = withDefaults(
  defineProps<{
    files: DiffFileModel[];
    projectPath: string;
    gitRef: string;
    scope: string;
    headSha: string;
    diffRefs?: DiffRefs | null;
    interactive?: boolean;
    /** 比较基准分支，用于工具栏文案。 */
    baseBranch?: string;
    /** MR 声称的变更文件总数；比已加载多时提示「还有 N 个未加载」。 */
    totalChanges?: number;
    /** 单文件都拉不动而被跳过的数量。 */
    skipped?: number;
    loading?: boolean;
  }>(),
  {
    diffRefs: null,
    interactive: false,
    baseBranch: "",
    totalChanges: 0,
    skipped: 0,
    loading: false,
  },
);

const emit = defineEmits<{ comment: [position: CommentPosition]; retry: [] }>();

interface TreeRow {
  key: string;
  type: "folder" | "file";
  depth: number;
  name: string;
  path: string;
  collapsed: boolean;
  file?: DiffFileModel;
  additions: number;
  deletions: number;
}

const query = ref("");
const layout = ref<"tree" | "list">("tree");
const collapsedFolders = ref<string[]>([]);
const activePath = ref("");
const viewed = ref<string[]>(loadViewed(props.scope));
const noticeDismissed = ref(false);

const tree = computed(() => buildDiffTree(props.files));
const filteredTree = computed(() => filterDiffTree(tree.value, query.value));
const visibleFiles = computed(() => flattenDiffTree(filteredTree.value));
const totals = computed(() => diffTotals(props.files));
const done = computed(() => props.files.filter((file) => isViewed(file)).length);
const missing = computed(() => diffMissingFiles(props.totalChanges, props.files.length));
const showNotice = computed(
  () => !noticeDismissed.value && (missing.value > 0 || props.skipped > 0),
);

const activeFile = computed<DiffFileModel | null>(() => {
  const list = visibleFiles.value;
  if (!list.length) return null;
  return list.find((file) => fileKey(file) === activePath.value) ?? list[0];
});

const activeIndex = computed(() => {
  const file = activeFile.value;
  if (!file) return -1;
  return visibleFiles.value.findIndex((item) => fileKey(item) === fileKey(file));
});

const rows = computed<TreeRow[]>(() => {
  const keywords = query.value.trim();
  const build = (nodes: DiffTreeNode[], depth: number): TreeRow[] => {
    const result: TreeRow[] = [];
    for (const node of nodes) {
      if (node.folder) {
        // 搜索时强制展开，否则命中的文件会被折叠目录藏起来。
        const collapsed = !keywords && collapsedFolders.value.includes(node.path);
        result.push({
          key: `folder:${node.path}`,
          type: "folder",
          depth,
          name: node.name,
          path: node.path,
          collapsed,
          additions: 0,
          deletions: 0,
        });
        if (!collapsed) result.push(...build(node.children, depth + 1));
      } else if (node.file) {
        const stats = diffStats(node.file.diff ?? "");
        result.push({
          key: `file:${node.path}`,
          type: "file",
          depth,
          name: node.name,
          path: node.path,
          collapsed: false,
          file: node.file,
          additions: stats.additions,
          deletions: stats.deletions,
        });
      }
    }
    return result;
  };
  return build(filteredTree.value, 0);
});

const listRows = computed<TreeRow[]>(() =>
  visibleFiles.value.map((file) => {
    const stats = diffStats(file.diff ?? "");
    return {
      key: `file:${fileKey(file)}`,
      type: "file" as const,
      depth: 0,
      name: fileKey(file),
      path: fileKey(file),
      collapsed: false,
      file,
      additions: stats.additions,
      deletions: stats.deletions,
    };
  }),
);

const displayRows = computed(() => (layout.value === "tree" ? rows.value : listRows.value));

watch(
  () => props.scope,
  (scope) => {
    viewed.value = loadViewed(scope);
  },
);

watch(
  () => props.files,
  () => {
    if (!props.files.some((file) => fileKey(file) === activePath.value)) {
      activePath.value = "";
    }
  },
);

// 又加载了一批文件后，之前关掉的提示条要能重新出现。
watch(
  () => props.files.length,
  () => {
    noticeDismissed.value = false;
  },
);

function isViewed(file: DiffFileModel): boolean {
  return viewed.value.includes(viewedKey(props.headSha, fileKey(file)));
}

function setViewed(file: DiffFileModel, value: boolean): void {
  const key = viewedKey(props.headSha, fileKey(file));
  const next = value
    ? viewed.value.includes(key)
      ? viewed.value
      : [...viewed.value, key]
    : viewed.value.filter((item) => item !== key);
  viewed.value = next;
  saveViewed(props.scope, next);
}

function selectFile(file: DiffFileModel): void {
  activePath.value = fileKey(file);
}

function isActive(file: DiffFileModel): boolean {
  return activeFile.value ? fileKey(activeFile.value) === fileKey(file) : false;
}

function toggleFolder(path: string): void {
  collapsedFolders.value = collapsedFolders.value.includes(path)
    ? collapsedFolders.value.filter((item) => item !== path)
    : [...collapsedFolders.value, path];
}

/** 「展开所有文件」= 退出折叠 + 重新拉一次（缺失的文件可能是分页没取完）。 */
function expandAll(): void {
  collapsedFolders.value = [];
  noticeDismissed.value = true;
  emit("retry");
}

function toggleLayout(): void {
  layout.value = layout.value === "tree" ? "list" : "tree";
}

function onViewedChange(value: boolean): void {
  const file = activeFile.value;
  if (file) setViewed(file, value);
}
</script>

<template>
  <div class="gl-diff-browser">
    <div class="gl-diff-toolbar">
      <span class="gl-diff-base">
        <Icon name="branch" :size="14" />
        <span class="muted">{{ t("diffCompareBase") }}</span>
        <code>{{ baseBranch || "-" }}</code>
      </span>
      <span class="grow"></span>
      <span class="muted small">{{ t("diffFileCount", { count: files.length }) }}</span>
      <span class="diff-stat">
        <b class="add">+{{ totals.additions }}</b>
        <b class="del">-{{ totals.deletions }}</b>
      </span>
      <button
        class="gl-icon-btn"
        type="button"
        :title="t('diffExpandAll')"
        :aria-label="t('diffExpandAll')"
        @click="expandAll"
      >
        <Icon name="chevron-down" :size="15" />
      </button>
      <button
        class="gl-icon-btn"
        type="button"
        :title="layout === 'tree' ? t('diffListView') : t('diffTreeView')"
        :aria-label="layout === 'tree' ? t('diffListView') : t('diffTreeView')"
        @click="toggleLayout"
      >
        <Icon :name="layout === 'tree' ? 'list' : 'folder'" :size="15" />
      </button>
    </div>

    <div v-if="showNotice" class="gl-diff-notice">
      <Icon name="warning" :size="16" />
      <div class="gl-diff-notice-main">
        <strong>{{ t("diffPartialTitle") }}</strong>
        <p class="muted small">{{ t("diffPartialHint", { count: missing }) }}</p>
        <p v-if="skipped" class="muted small">
          {{ t("diffSkippedHint", { count: skipped }) }}
        </p>
      </div>
      <button class="btn small" type="button" @click="expandAll">{{ t("diffExpandAll") }}</button>
      <button class="gl-icon-btn" type="button" :aria-label="t('close')" @click="noticeDismissed = true">
        <Icon name="x" :size="14" />
      </button>
    </div>

    <div class="gl-diff-columns">
      <aside class="gl-diff-files">
        <header class="gl-diff-files-head">
          <strong>{{ t("diffFileCount", { count: files.length }) }}</strong>
          <span class="muted small">{{ t("viewedProgress", { done, total: files.length }) }}</span>
          <span class="grow"></span>
          <button
            class="gl-icon-btn"
            type="button"
            :class="{ active: layout === 'tree' }"
            :title="t('diffTreeView')"
            :aria-label="t('diffTreeView')"
            @click="layout = 'tree'"
          >
            <Icon name="folder" :size="14" />
          </button>
          <button
            class="gl-icon-btn"
            type="button"
            :class="{ active: layout === 'list' }"
            :title="t('diffListView')"
            :aria-label="t('diffListView')"
            @click="layout = 'list'"
          >
            <Icon name="list" :size="14" />
          </button>
        </header>

        <label class="gl-diff-search search-field">
          <Icon name="search" :size="14" />
          <input
            v-model="query"
            type="search"
            :placeholder="t('diffSearchPlaceholder')"
            :aria-label="t('diffSearchPlaceholder')"
          />
        </label>

        <div class="gl-diff-tree">
          <p v-if="loading && !files.length" class="loading">{{ t("loading") }}</p>
          <p v-else-if="!displayRows.length" class="empty">{{ t("noSearchResults") }}</p>
          <template v-else>
            <template v-for="row in displayRows" :key="row.key">
              <button
                v-if="row.type === 'folder'"
                class="gl-diff-folder"
                type="button"
                :style="{ paddingLeft: `${8 + row.depth * 12}px` }"
                @click="toggleFolder(row.path)"
              >
                <Icon :name="row.collapsed ? 'chevron' : 'chevron-down'" :size="12" />
                <Icon name="folder" :size="14" />
                <span>{{ row.name }}</span>
              </button>
              <button
                v-else-if="row.file"
                class="gl-diff-file"
                type="button"
                :class="{ active: isActive(row.file), viewed: isViewed(row.file) }"
                :style="{ paddingLeft: `${8 + row.depth * 12}px` }"
                @click="selectFile(row.file)"
              >
                <Icon name="file" :size="14" />
                <span class="gl-diff-file-name">{{ row.name }}</span>
                <span class="diff-stat">
                  <b class="add">+{{ row.additions }}</b>
                  <b class="del">-{{ row.deletions }}</b>
                </span>
              </button>
            </template>
          </template>
        </div>
      </aside>

      <section class="gl-diff-content">
        <DiffFile
          v-if="activeFile"
          :key="fileKey(activeFile)"
          :file="activeFile"
          :project-path="projectPath"
          :git-ref="gitRef"
          :diff-refs="diffRefs"
          :interactive="interactive"
          :viewed="isViewed(activeFile)"
          :collapsible="false"
          @comment="(position) => emit('comment', position)"
          @update:viewed="onViewedChange"
        />
        <p v-else class="empty">{{ t("diffSelectFile") }}</p>
      </section>
    </div>
  </div>
</template>
