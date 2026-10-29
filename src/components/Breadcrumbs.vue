<script setup lang="ts">
import type { Crumb, CrumbId, Route } from "../lib/nav";
import { t, type MessageKey } from "../i18n";
import Icon from "./Icon.vue";

defineProps<{ crumbs: Crumb[] }>();
const emit = defineEmits<{ navigate: [route: Route] }>();

const LABELS: Record<CrumbId, MessageKey> = {
  projects: "tabProjects",
  project: "tabProjects",
  commits: "commits",
  commit: "commit",
  branches: "branches",
  branch: "branches",
  branchNew: "branchNew",
  mrs: "mergeRequests",
  mr: "mergeRequests",
  mrNew: "mrNewCrumb",
  pipelines: "pipelines",
  pipeline: "pipelines",
  issues: "issues",
  issue: "issues",
  todos: "todos",
};

function label(crumb: Crumb): string {
  return crumb.text ?? t(LABELS[crumb.id]);
}

function navigate(crumb: Crumb): void {
  if (crumb.route) emit("navigate", crumb.route);
}
</script>

<template>
  <nav class="breadcrumbs" aria-label="Breadcrumb">
    <template v-for="(crumb, index) in crumbs" :key="`${crumb.id}-${index}`">
      <Icon v-if="index > 0" class="crumb-sep" name="chevron" :size="12" />
      <button
        v-if="crumb.route"
        class="crumb-link"
        type="button"
        @click="navigate(crumb)"
      >
        {{ label(crumb) }}
      </button>
      <span v-else class="crumb-current" :aria-current="index === crumbs.length - 1 ? 'page' : undefined">
        {{ label(crumb) }}
      </span>
    </template>
  </nav>
</template>
