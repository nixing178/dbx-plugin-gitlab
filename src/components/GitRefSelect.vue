<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { t } from "../i18n";
import type { Branch } from "../types";
import Icon from "./Icon.vue";

/**
 * 可搜索的分支下拉。
 *
 * GitLab 网页用 GraphQL 搜分支，这里退化为 REST 的 `search` 参数：
 * 输入防抖 300ms 走服务端过滤，打开时预载最近 100 个分支。
 */
const props = withDefaults(
  defineProps<{
    projectId: string;
    modelValue: string;
    placeholder?: string;
    disabled?: boolean;
    /** 项目页浏览必须始终有分支，不允许清空。 */
    clearable?: boolean;
  }>(),
  { placeholder: "", disabled: false, clearable: true },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
  select: [branch: Branch | null];
}>();

const root = ref<HTMLElement | null>(null);
const open = ref(false);
const query = ref("");
const items = ref<Branch[]>([]);
const loading = ref(false);
const error = ref("");
let debounce: number | undefined;

const triggerLabel = computed(() => props.modelValue || props.placeholder || t("mrNewSelectBranch"));

async function load(): Promise<void> {
  if (!props.projectId) return;
  loading.value = true;
  error.value = "";
  try {
    const page = await gitlabApi.branches(props.projectId, query.value.trim(), 1, 100);
    items.value = page.items;
    const current = items.value.find((item) => item.name === props.modelValue);
    if (current) emit("select", current);
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

function onSearchInput(): void {
  window.clearTimeout(debounce);
  debounce = window.setTimeout(() => void load(), 300);
}

function toggle(): void {
  if (props.disabled) return;
  open.value = !open.value;
  if (open.value) {
    query.value = "";
    void load();
  }
}

function pick(branch: Branch): void {
  emit("update:modelValue", branch.name);
  emit("select", branch);
  open.value = false;
}

function clear(): void {
  emit("update:modelValue", "");
  emit("select", null);
  open.value = false;
}

function onDocMouseDown(event: MouseEvent): void {
  if (!root.value || !open.value) return;
  if (!root.value.contains(event.target as Node)) open.value = false;
}

watch(
  () => props.projectId,
  () => {
    items.value = [];
    open.value = false;
  },
);

onMounted(() => document.addEventListener("mousedown", onDocMouseDown));
onBeforeUnmount(() => {
  document.removeEventListener("mousedown", onDocMouseDown);
  window.clearTimeout(debounce);
});
</script>

<template>
  <div ref="root" class="gl-ref" :class="{ open }">
    <button
      class="gl-ref-trigger"
      type="button"
      :disabled="disabled"
      :aria-expanded="open"
      :title="triggerLabel"
      @click="toggle"
    >
      <Icon name="branch" :size="14" />
      <span class="gl-ref-value mono" :class="{ muted: !modelValue }">{{ triggerLabel }}</span>
      <Icon name="chevron-down" :size="12" />
    </button>

    <div v-if="open" class="gl-ref-menu" role="listbox">
      <label class="gl-ref-search">
        <Icon name="search" :size="14" />
        <input
          v-model="query"
          type="search"
          :placeholder="t('mrNewSearchBranch')"
          @input="onSearchInput"
          @keydown.esc="open = false"
        />
      </label>

      <div class="gl-ref-list">
        <div v-if="loading" class="gl-ref-note">{{ t("loading") }}</div>
        <div v-else-if="error" class="gl-ref-note error-text">{{ error }}</div>
        <template v-else>
          <button
            v-for="branch in items"
            :key="branch.name"
            class="gl-ref-option"
            type="button"
            :class="{ active: branch.name === modelValue }"
            @click="pick(branch)"
          >
            <span class="mono gl-ellipsis">{{ branch.name }}</span>
            <span v-if="branch.default" class="badge" data-tone="neutral">
              {{ t("defaultBranch") }}
            </span>
          </button>
          <div v-if="!items.length" class="gl-ref-note">{{ t("noBranches") }}</div>
        </template>
      </div>

      <button v-if="clearable && modelValue" class="gl-ref-clear" type="button" @click="clear">
        {{ t("mrNewSelectBranch") }}
      </button>
    </div>
  </div>
</template>
