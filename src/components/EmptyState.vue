<script setup lang="ts">
/**
 * GitLab 式空状态：插画 + 标题 + 说明 + 可选操作。
 * 插画自绘（不引第三方资源），配色跟随主题变量。
 */
defineProps<{ title: string; hint?: string }>();
</script>

<template>
  <div class="gl-empty-state">
    <svg class="gl-empty-art" viewBox="0 0 160 124" aria-hidden="true">
      <path class="gl-empty-link" d="M56 50c4 20 12 30 24 30h6" />
      <path class="gl-empty-link" d="M104 50c-4 20-12 30-24 30" />
      <circle class="gl-empty-node" cx="40" cy="34" r="21" />
      <circle class="gl-empty-node" cx="120" cy="34" r="21" />
      <circle class="gl-empty-node accent" cx="80" cy="86" r="21" />
      <path class="gl-empty-arrow" d="M94 66l10 8-10 8" />
    </svg>
    <h2 class="gl-empty-title">{{ title }}</h2>
    <p v-if="hint" class="gl-empty-hint">{{ hint }}</p>
    <div v-if="$slots.default" class="gl-empty-actions">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.gl-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 52px 24px 60px;
  text-align: center;
}

.gl-empty-art {
  width: 160px;
  height: 120px;
}

.gl-empty-node {
  fill: var(--panel-soft);
  stroke: var(--border);
  stroke-width: 2;
}

.gl-empty-node.accent {
  fill: var(--accent-soft);
  stroke: var(--accent);
}

.gl-empty-link {
  fill: none;
  stroke: var(--border);
  stroke-width: 2;
  stroke-dasharray: 4 4;
}

.gl-empty-arrow {
  fill: none;
  stroke: var(--accent);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.gl-empty-title {
  margin: 6px 0 0;
  font-size: 18px;
  font-weight: 600;
}

.gl-empty-hint {
  margin: 0;
  max-width: 440px;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
}

.gl-empty-actions {
  margin-top: 10px;
  display: flex;
  gap: 8px;
}
</style>
