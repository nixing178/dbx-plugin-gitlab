<script setup lang="ts">
import Icon from "./Icon.vue";

/**
 * 侧栏一行：主按钮负责跳转，右侧钉选按钮单独成键。
 * 不能把钉选按钮塞进主按钮内部（HTML 不允许多层 button），所以外面套一层定位容器。
 */
withDefaults(
  defineProps<{
    label: string;
    icon: string;
    active?: boolean;
    pinned?: boolean;
    collapsed?: boolean;
    pinTitle?: string;
  }>(),
  { active: false, pinned: false, collapsed: false, pinTitle: "" },
);

const emit = defineEmits<{ open: []; togglePin: [] }>();
</script>

<template>
  <div class="sidebar-row">
    <button
      class="sidebar-item"
      type="button"
      :class="{ active }"
      :title="collapsed ? label : undefined"
      :aria-current="active ? 'page' : undefined"
      @click="emit('open')"
    >
      <Icon :name="icon" />
      <span class="sidebar-label">{{ label }}</span>
    </button>
    <button
      v-if="!collapsed"
      class="sidebar-pin"
      :class="{ on: pinned }"
      type="button"
      :title="pinTitle"
      :aria-label="pinTitle"
      :aria-pressed="pinned"
      @click.stop="emit('togglePin')"
    >
      <Icon name="pin" :size="14" />
    </button>
  </div>
</template>
