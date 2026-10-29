<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { t } from "../i18n";
import Icon from "./Icon.vue";

/** 下拉选项：标签用 color/textColor，人用 avatar。 */
interface GitSelectOption {
  value: string;
  label: string;
  hint?: string;
  color?: string;
  textColor?: string;
  avatar?: string;
}

/**
 * GitLab 风格的下拉：单选（里程碑）与多选（指派人 / 审核者 / 标记）共用。
 *
 * 选项由父组件提供（成员、标签、里程碑来自不同 RPC），这里只负责
 * 开合、本地过滤、多选回显与「清空」。
 */
const props = withDefaults(
  defineProps<{
    placeholder: string;
    options: GitSelectOption[];
    mode?: "single" | "multiple";
    value?: string;
    values?: string[];
    loading?: boolean;
    emptyText?: string;
    disabled?: boolean;
  }>(),
  {
    mode: "single",
    value: "",
    values: () => [],
    loading: false,
    emptyText: "",
    disabled: false,
  },
);

const emit = defineEmits<{
  "update:value": [value: string];
  "update:values": [values: string[]];
}>();

const root = ref<HTMLElement | null>(null);
const open = ref(false);
const query = ref("");

const multiple = computed(() => props.mode === "multiple");
const selectedValues = computed(() => (multiple.value ? props.values : props.value ? [props.value] : []));
const selectedOptions = computed(() =>
  selectedValues.value
    .map((value) => props.options.find((option) => option.value === value))
    .filter((option): option is GitSelectOption => Boolean(option)),
);

const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase();
  if (!keyword) return props.options;
  return props.options.filter(
    (option) =>
      option.label.toLowerCase().includes(keyword) ||
      (option.hint ?? "").toLowerCase().includes(keyword),
  );
});

function isSelected(value: string): boolean {
  return selectedValues.value.includes(value);
}

function toggleOpen(): void {
  if (props.disabled) return;
  open.value = !open.value;
  if (open.value) query.value = "";
}

function choose(option: GitSelectOption): void {
  if (multiple.value) {
    const current = selectedValues.value;
    const next = current.includes(option.value)
      ? current.filter((value) => value !== option.value)
      : [...current, option.value];
    emit("update:values", next);
    return;
  }
  emit("update:value", option.value);
  open.value = false;
}

function clear(): void {
  if (multiple.value) {
    emit("update:values", []);
  } else {
    emit("update:value", "");
  }
  open.value = false;
}

function onDocMouseDown(event: MouseEvent): void {
  if (!root.value || !open.value) return;
  if (!root.value.contains(event.target as Node)) open.value = false;
}

watch(
  () => props.disabled,
  (disabled) => {
    if (disabled) open.value = false;
  },
);

onMounted(() => document.addEventListener("mousedown", onDocMouseDown));
onBeforeUnmount(() => document.removeEventListener("mousedown", onDocMouseDown));
</script>

<template>
  <div ref="root" class="gl-select" :class="{ open }">
    <button
      class="gl-select-trigger"
      type="button"
      :disabled="disabled"
      :aria-expanded="open"
      @click="toggleOpen"
    >
      <span v-if="!selectedOptions.length" class="muted">{{ placeholder }}</span>
      <span v-else class="gl-select-values">
        <span
          v-for="option in selectedOptions"
          :key="option.value"
          class="gl-select-chip"
          :style="option.color ? { background: option.color, color: option.textColor ?? '#fff' } : undefined"
        >
          <span v-if="option.avatar" class="gl-select-avatar">{{ option.avatar }}</span>
          {{ option.label }}
        </span>
      </span>
      <Icon name="chevron-down" :size="12" />
    </button>

    <div v-if="open" class="gl-select-menu" role="listbox" :aria-multiselectable="multiple">
      <label class="gl-select-search">
        <Icon name="search" :size="14" />
        <input v-model="query" type="search" :placeholder="placeholder" @keydown.esc="open = false" />
      </label>

      <div class="gl-select-list">
        <div v-if="loading" class="gl-select-note">{{ t("loading") }}</div>
        <template v-else>
          <button
            v-for="option in filtered"
            :key="option.value"
            class="gl-select-option"
            type="button"
            :class="{ active: isSelected(option.value) }"
            @click="choose(option)"
          >
            <span class="gl-select-mark">
              <Icon v-if="isSelected(option.value)" name="check" :size="13" />
            </span>
            <span v-if="option.color && !option.avatar" class="gl-label-dot" :style="{ background: option.color }"></span>
            <span class="gl-select-label">
              <span>{{ option.label }}</span>
              <span v-if="option.hint" class="gl-select-hint">{{ option.hint }}</span>
            </span>
          </button>
          <div v-if="!filtered.length" class="gl-select-note">{{ emptyText || t("noData") }}</div>
        </template>
      </div>

      <button v-if="selectedValues.length" class="gl-select-clear" type="button" @click="clear">
        {{ t("mrNewClearSelection") }}
      </button>
    </div>
  </div>
</template>
