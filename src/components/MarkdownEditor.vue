<script setup lang="ts">
import { nextTick, ref } from "vue";
import { t } from "../i18n";
import MarkdownBlock from "./MarkdownBlock.vue";

/**
 * GitLab 风格描述编辑器：Markdown 文本 + 预览双态。
 *
 * 表格 / 附件这类需要宿主能力的入口不做假按钮：
 * 附件上传在插件沙箱里没有落点，因此不提供。
 */
const props = withDefaults(
  defineProps<{ modelValue: string; placeholder?: string; compact?: boolean }>(),
  { placeholder: "", compact: false },
);
const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const textarea = ref<HTMLTextAreaElement | null>(null);
const mode = ref<"write" | "preview">("write");

function replaceSelection(transform: (selected: string) => string, cursorOffset = 0): void {
  const el = textarea.value;
  if (!el) return;
  const start = el.selectionStart;
  const end = el.selectionEnd;
  const selected = props.modelValue.slice(start, end);
  const next = `${props.modelValue.slice(0, start)}${transform(selected)}${props.modelValue.slice(end)}`;
  emit("update:modelValue", next);
  void nextTick(() => {
    el.focus();
    const caret = start + cursorOffset;
    el.selectionStart = caret;
    el.selectionEnd = caret + selected.length;
  });
}

function wrap(before: string, after = before, placeholder = ""): void {
  replaceSelection((selected) => `${before}${selected || placeholder}${after}`, before.length);
}

function prefixLines(prefix: string | ((index: number) => string)): void {
  const el = textarea.value;
  if (!el) return;
  const start = el.selectionStart;
  const end = el.selectionEnd;
  const selected = props.modelValue.slice(start, end) || "";
  const lines = selected ? selected.split("\n") : [""];
  const prefixed = lines
    .map((line, index) => `${typeof prefix === "function" ? prefix(index) : prefix}${line}`)
    .join("\n");
  const next = `${props.modelValue.slice(0, start)}${prefixed}${props.modelValue.slice(end)}`;
  emit("update:modelValue", next);
  void nextTick(() => {
    el.focus();
    el.selectionStart = start;
    el.selectionEnd = start + prefixed.length;
  });
}

function insertTable(): void {
  wrap("", "", "| 列 1 | 列 2 |\n| --- | --- |\n| 值 | 值 |");
  const el = textarea.value;
  if (!el) return;
  const start = el.selectionStart;
  const end = el.selectionStart + "| 列 1 | 列 2 |\n| --- | --- |\n| 值 | 值 |".length;
  void nextTick(() => {
    el.focus();
    el.selectionStart = start;
    el.selectionEnd = end;
  });
}

interface EditorTool {
  key: string;
  label: string;
  title: string;
  run: () => void;
}

const tools: EditorTool[] = [
  { key: "bold", label: "B", title: "Bold", run: () => wrap("**", "**", "粗体") },
  { key: "italic", label: "I", title: "Italic", run: () => wrap("_", "_", "斜体") },
  { key: "link", label: "链接", title: "Link", run: () => wrap("[", "](https://)", "文本") },
  { key: "quote", label: "引用", title: "Quote", run: () => prefixLines("> ") },
  { key: "code", label: "代码", title: "Code", run: () => wrap("`", "`", "code") },
  { key: "ul", label: "列表", title: "Bulleted list", run: () => prefixLines("- ") },
  { key: "ol", label: "1.", title: "Numbered list", run: () => prefixLines((i) => `${i + 1}. `) },
  { key: "task", label: "任务", title: "Task list", run: () => prefixLines("- [ ] ") },
  { key: "table", label: "表格", title: "Table", run: insertTable },
];

function onInput(event: Event): void {
  emit("update:modelValue", (event.target as HTMLTextAreaElement).value);
}

function toggleMode(): void {
  mode.value = mode.value === "preview" ? "write" : "preview";
}
</script>

<template>
  <div class="gl-md" :class="{ 'gl-md-compact': compact }">
    <div class="gl-md-toolbar">
      <button
        class="gl-md-tool"
        type="button"
        :class="{ active: mode === 'preview' }"
        @click="toggleMode"
      >
        {{ mode === "preview" ? t("mrNewWrite") : t("mrNewPreview") }}
      </button>
      <span class="gl-md-sep"></span>
      <button
        v-for="tool in tools"
        :key="tool.key"
        class="gl-md-tool"
        type="button"
        :title="tool.title"
        @click="tool.run"
      >
        {{ tool.label }}
      </button>
    </div>

    <textarea
      v-if="mode === 'write'"
      ref="textarea"
      class="gl-md-textarea"
      :rows="compact ? 5 : 8"
      :value="modelValue"
      :placeholder="placeholder || t('mrNewDescriptionHint')"
      @input="onInput"
    ></textarea>
    <div v-else class="gl-md-preview">
      <MarkdownBlock v-if="modelValue.trim()" :source="modelValue" />
      <p v-else class="muted">{{ t("noData") }}</p>
    </div>

    <div class="gl-md-foot">
      <button class="link-btn" type="button" @click="toggleMode">
        {{ mode === "preview" ? t("mrNewSwitchToText") : t("mrNewPreview") }}
      </button>
      <span class="gl-md-foot-right">
        <slot name="actions" />
        <span v-if="!compact" class="gl-md-badge">MD</span>
      </span>
    </div>
  </div>
</template>
