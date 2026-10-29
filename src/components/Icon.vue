<script setup lang="ts">
import { computed } from "vue";

/** 内联图标集：不引第三方图标库，单文件产物里也不多一把依赖。 */
const ICONS: Record<string, string[]> = {
  home: ["M3 11 12 3l9 8", "M6 10v10h12V10"],
  folder: ["M3 7a1 1 0 0 1 1-1h5l2 2h9a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1z"],
  file: ["M6 3h8l4 4v14H6z", "M14 3v5h5"],
  "file-text": ["M6 3h8l4 4v14H6z", "M14 3v5h5", "M9 13h6", "M9 17h4"],
  "file-code": ["M6 3h8l4 4v14H6z", "M14 3v5h5", "M10.5 12.5 9 14l1.5 1.5", "M13.5 12.5 15 14l-1.5 1.5"],
  "file-image": ["M6 3h8l4 4v14H6z", "M14 3v5h5", "M9 18l2.6-3.4 1.8 2.1L15 15l2 3"],
  branch: ["M6 3v12", "M9 18a3 3 0 1 1-6 0 3 3 0 0 1 6 0", "M18 9a3 3 0 1 1-6 0 3 3 0 0 1 6 0", "M18 9v1a6 6 0 0 1-6 6H9"],
  merge: ["M6 3v12", "M9 18a3 3 0 1 1-6 0 3 3 0 0 1 6 0", "M18 9a3 3 0 1 1-6 0 3 3 0 0 1 6 0", "M18 9v2a4 4 0 0 1-4 4H8"],
  issue: ["M9 12a3 3 0 1 0 6 0 3 3 0 0 0-6 0", "M3 12a9 9 0 1 0 18 0 9 9 0 0 0-18 0"],
  pipeline: ["M4 7h6v4H4z", "M14 7h6v4h-6z", "M9 9h2", "M4 15h6v4H4z", "M14 15h6v4h-6z", "M9 17h2"],
  check: ["M4 12l5 5L20 6"],
  x: ["M5 5l14 14", "M19 5L5 19"],
  clock: ["M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18", "M12 7v5l3 2"],
  search: ["M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16", "M21 21l-4.3-4.3"],
  chevron: ["M9 6l6 6-6 6"],
  "chevron-up": ["M6 15l6-6 6 6"],
  "chevron-down": ["M6 9l6 6 6-6"],
  "chevron-left": ["M15 6l-6 6 6 6"],
  menu: ["M4 7h16", "M4 12h16", "M4 17h16"],
  users: [
    "M16 20v-2a4 4 0 0 0-4-4H7a4 4 0 0 0-4 4v2",
    "M9.5 10a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7",
    "M17 10a3.5 3.5 0 1 0 0-7",
    "M21 20v-2a3.5 3.5 0 0 0-3-3.4",
  ],
  tag: ["M3 12V5a2 2 0 0 1 2-2h7l9 9-9 9z", "M8 8h.01"],
  play: ["M8 5l11 7-11 7z"],
  list: ["M8 6h13", "M8 12h13", "M8 18h13", "M3 6h.01", "M3 12h.01", "M3 18h.01"],
  code: ["M9 18l-6-6 6-6", "M15 6l6 6-6 6"],
  lock: ["M7 11V8a5 5 0 0 1 10 0v3", "M5 11h14v10H5z"],
  star: ["M12 4l2.4 5 5.6.8-4 3.9.9 5.5-4.9-2.6-4.9 2.6.9-5.5-4-3.9 5.6-.8z"],
  fork: ["M6 4v8", "M18 4v3a4 4 0 0 1-4 4h-4", "M9 20a3 3 0 1 1-6 0 3 3 0 0 1 6 0", "M21 6a3 3 0 1 1-6 0 3 3 0 0 1 6 0"],
  dot: ["M12 13a1 1 0 1 0 0-2 1 1 0 0 0 0 2"],
  commit: ["M4 12h5", "M15 12h5", "M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6"],
  disk: ["M4 5h16v14H4z", "M8 5v5h8V5", "M8 19v-5h8v5"],
  external: ["M14 4h6v6", "M20 4l-9 9", "M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"],
  funnel: ["M4 5h16l-6 8.5V18l-4 2v-6.5z"],
  sort: ["M8 4v16", "M5 17l3 3 3-3", "M16 20V4", "M13 7l3-3 3 3"],
  refresh: ["M20 12a8 8 0 1 1-2.5-5.8", "M20 4v5h-5"],
  more: ["M12 6h.01", "M12 12h.01", "M12 18h.01"],
};

const props = withDefaults(defineProps<{ name: string; size?: number }>(), { size: 16 });

const paths = computed(() => ICONS[props.name] ?? []);
</script>

<template>
  <svg
    class="icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path v-for="(d, index) in paths" :key="index" :d="d" />
  </svg>
</template>

<style scoped>
.icon {
  flex: 0 0 auto;
  display: block;
}
</style>
