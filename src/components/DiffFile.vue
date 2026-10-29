<script setup lang="ts">
import { computed, ref } from "vue";
import { describeError, gitlabApi } from "../api";
import { t } from "../i18n";
import {
  diffStats,
  expandContext,
  parseDiff,
  splitHunks,
  type DiffLine,
  type HunkRange,
} from "../lib/diff";
import { inlinePosition } from "../lib/position";
import DiffRow from "./DiffRow.vue";
import Icon from "./Icon.vue";
import type { CommentPosition, DiffFile, DiffRefs } from "../types";

const props = withDefaults(
  defineProps<{
    file: DiffFile;
    projectPath: string;
    gitRef: string;
    diffRefs?: DiffRefs | null;
    interactive?: boolean;
    viewed?: boolean;
    /** 提交详情这类不支持「标记已查看」的页面把它关掉，避免出现无效勾选框。 */
    showViewed?: boolean;
    /** 单文件视图（变更页右栏）里折叠按钮没有意义。 */
    collapsible?: boolean;
  }>(),
  { diffRefs: null, interactive: false, viewed: false, showViewed: true, collapsible: true },
);

const emit = defineEmits<{
  comment: [position: CommentPosition];
  "update:viewed": [value: boolean];
}>();

const STEP = 10;

/** 同一文件被多个组件实例展开时共用一份内容，避免重复请求。 */
const fileCache = new Map<string, string[]>();

const collapsed = ref(false);
const lines = computed(() => parseDiff(props.file.diff ?? ""));
const blocks = computed(() => splitHunks(lines.value));
const stats = computed(() => diffStats(props.file.diff ?? ""));
const path = computed(() => props.file.new_path || props.file.old_path);
const expansions = ref<Record<number, { before: number; after: number }>>({});
const fileLines = ref<string[] | null>(null);
const loadingContext = ref(false);
const contextError = ref("");

const flag = computed(() => {
  if (props.file.new_file) return t("fileNew");
  if (props.file.deleted_file) return t("fileDeleted");
  if (props.file.renamed_file) return t("fileRenamed");
  return "";
});

const expandedMap = computed<Record<number, { before: DiffLine[]; after: DiffLine[] }>>(() => {
  const map: Record<number, { before: DiffLine[]; after: DiffLine[] }> = {};
  const source = fileLines.value;
  if (!source) return map;
  blocks.value.forEach((block, index) => {
    const counts = expansions.value[index];
    if (!counts || !block.range) return;
    map[index] = expandContext(source, block.range, counts.before, counts.after);
  });
  return map;
});

function lastNewLine(range: HunkRange): number {
  return range.newStart + range.newCount - 1;
}

function hasBefore(range: HunkRange | null, index: number): boolean {
  if (!range) return false;
  const shown = expandedMap.value[index]?.before.length ?? 0;
  return shown < range.newStart - 1;
}

function hasAfter(range: HunkRange | null, index: number): boolean {
  if (!range) return false;
  if (!fileLines.value) return true;
  const shown = expandedMap.value[index]?.after.length ?? 0;
  return shown < fileLines.value.length - lastNewLine(range);
}

async function ensureFileLines(): Promise<string[] | null> {
  if (fileLines.value) return fileLines.value;
  const cacheKey = `${props.projectPath}@${props.gitRef}:${path.value}`;
  const cached = fileCache.get(cacheKey);
  if (cached) {
    fileLines.value = cached;
    return cached;
  }
  loadingContext.value = true;
  contextError.value = "";
  try {
    const content = await gitlabApi.file(props.projectPath, path.value, props.gitRef);
    if (content.too_large || content.binary || !content.text) {
      contextError.value = t("contextUnavailable");
      return null;
    }
    const parsed = content.text.replace(/\r\n/g, "\n").split("\n");
    fileCache.set(cacheKey, parsed);
    fileLines.value = parsed;
    return parsed;
  } catch (err) {
    contextError.value = describeError(err) || t("contextUnavailable");
    return null;
  } finally {
    loadingContext.value = false;
  }
}

async function expand(index: number, direction: "before" | "after"): Promise<void> {
  const range = blocks.value[index]?.range;
  if (!range) return;
  const ready = await ensureFileLines();
  if (!ready) return;
  const current = expansions.value[index] ?? { before: 0, after: 0 };
  expansions.value = {
    ...expansions.value,
    [index]: {
      before: direction === "before" ? current.before + STEP : current.before,
      after: direction === "after" ? current.after + STEP : current.after,
    },
  };
}

function canComment(line: DiffLine): boolean {
  if (!props.interactive || !props.diffRefs) return false;
  if (line.kind !== "add" && line.kind !== "del" && line.kind !== "context") return false;
  return line.new_line !== null || line.old_line !== null;
}

function commentOn(line: DiffLine): void {
  if (!props.diffRefs) return;
  const position = inlinePosition(props.file, line, props.diffRefs);
  if (position) emit("comment", position);
}

function onViewedChange(event: Event): void {
  emit("update:viewed", (event.target as HTMLInputElement).checked);
}
</script>

<template>
  <section class="diff-file" :class="{ 'is-viewed': viewed }">
    <header class="diff-head">
      <button
        v-if="collapsible"
        class="diff-toggle"
        type="button"
        @click="collapsed = !collapsed"
      >
        <Icon :name="collapsed ? 'chevron' : 'chevron-down'" :size="14" />
      </button>
      <span class="path">{{ path }}</span>
      <span v-if="flag" class="badge" data-tone="info">{{ flag }}</span>
      <span class="diff-stat">
        <b class="add">+{{ stats.additions }}</b>
        <b class="del">-{{ stats.deletions }}</b>
      </span>
      <label v-if="showViewed" class="viewed-box">
        <input type="checkbox" :checked="viewed" @change="onViewedChange" />
        <span>{{ t("viewed") }}</span>
      </label>
    </header>

    <div v-if="!collapsible || !collapsed">
      <p v-if="file.too_large" class="empty">{{ t("diffTooLarge") }}</p>
      <p v-else-if="!file.diff" class="empty">{{ t("diffEmpty") }}</p>
      <p v-else-if="contextError" class="muted small diff-note">{{ contextError }}</p>

      <div v-if="file.diff && !file.too_large" class="diff-body">
        <template v-for="(block, blockIndex) in blocks" :key="blockIndex">
          <template v-if="block.header">
            <div class="diff-hunk-bar">
              <button
                v-if="hasBefore(block.range, blockIndex)"
                class="hunk-btn"
                type="button"
                :disabled="loadingContext"
                @click="expand(blockIndex, 'before')"
              >
                ↑ {{ t("expandBefore") }}
              </button>
              <span class="hunk-label mono">{{ block.header.text }}</span>
              <button
                v-if="hasAfter(block.range, blockIndex)"
                class="hunk-btn"
                type="button"
                :disabled="loadingContext"
                @click="expand(blockIndex, 'after')"
              >
                ↓ {{ t("expandAfter") }}
              </button>
            </div>

            <DiffRow
              v-for="(line, index) in expandedMap[blockIndex]?.before ?? []"
              :key="`b-${blockIndex}-${index}`"
              :line="line"
              :can-comment="canComment(line)"
              @comment="commentOn(line)"
            />
            <DiffRow
              v-for="(line, index) in block.lines"
              :key="`m-${blockIndex}-${index}`"
              :line="line"
              :can-comment="canComment(line)"
              @comment="commentOn(line)"
            />
            <DiffRow
              v-for="(line, index) in expandedMap[blockIndex]?.after ?? []"
              :key="`a-${blockIndex}-${index}`"
              :line="line"
              :can-comment="canComment(line)"
              @comment="commentOn(line)"
            />
          </template>

          <template v-else>
            <DiffRow
              v-for="(line, index) in block.lines"
              :key="`p-${blockIndex}-${index}`"
              :line="line"
              :can-comment="canComment(line)"
              @comment="commentOn(line)"
            />
          </template>
        </template>
      </div>
    </div>
  </section>
</template>
