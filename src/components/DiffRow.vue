<script setup lang="ts">
import type { DiffLine } from "../lib/diff";

defineProps<{ line: DiffLine; canComment?: boolean }>();
const emit = defineEmits<{ comment: [] }>();

function prefix(kind: string): string {
  if (kind === "add") return "+";
  if (kind === "del") return "-";
  if (kind === "context") return " ";
  return "";
}
</script>

<template>
  <div class="diff-line" :class="line.kind">
    <span class="ln">{{ line.old_line ?? "" }}</span>
    <span class="ln">{{ line.new_line ?? "" }}</span>
    <span class="code">{{ prefix(line.kind) }}{{ line.text }}</span>
    <button
      v-if="canComment"
      class="comment-btn"
      type="button"
      title="+"
      @click.stop="emit('comment')"
    >
      +
    </button>
  </div>
</template>
