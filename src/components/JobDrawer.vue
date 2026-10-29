<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { t } from "../i18n";
import JobLog from "./JobLog.vue";
import type { GitLabProject, Job } from "../types";

const props = withDefaults(
  defineProps<{ project: GitLabProject; job: Job; readOnly?: boolean }>(),
  { readOnly: true },
);
const emit = defineEmits<{ close: []; changed: [] }>();

const MIN_WIDTH = 380;
const MAX_WIDTH = 1100;
const width = ref(560);
const dragging = ref(false);

function startDrag(event: PointerEvent): void {
  dragging.value = true;
  (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
}

function onDrag(event: PointerEvent): void {
  if (!dragging.value) return;
  const next = window.innerWidth - event.clientX;
  width.value = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, next));
}

function stopDrag(): void {
  dragging.value = false;
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === "Escape") emit("close");
}

onMounted(() => window.addEventListener("keydown", onKeydown));
onBeforeUnmount(() => window.removeEventListener("keydown", onKeydown));
</script>

<template>
  <div class="drawer-shell" @click.self="emit('close')">
    <aside class="job-drawer" :style="{ width: `${width}px` }">
      <div
        class="drawer-resize"
        :class="{ active: dragging }"
        @pointerdown.prevent="startDrag"
        @pointermove="onDrag"
        @pointerup="stopDrag"
        @pointercancel="stopDrag"
      ></div>

      <header class="drawer-head">
        <strong>{{ t("jobLog") }}</strong>
        <span class="mono muted small">#{{ job.id }}</span>
        <span class="grow"></span>
        <button class="btn ghost small" type="button" :title="t('close')" @click="emit('close')">
          ×
        </button>
      </header>

      <div class="drawer-body">
        <JobLog
          :key="job.id"
          :project="project"
          :job="job"
          :read-only="readOnly"
          @changed="emit('changed')"
        />
      </div>
    </aside>
  </div>
</template>
