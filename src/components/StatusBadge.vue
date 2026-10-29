<script setup lang="ts">
import { computed } from "vue";
import { t, type MessageKey } from "../i18n";
import { statusKey, statusTone } from "../lib/status";

const props = defineProps<{ status: string }>();

const tone = computed(() => statusTone(props.status));
const label = computed(() => t(statusKey(props.status) as MessageKey));
</script>

<template>
  <span class="badge" :data-tone="tone">{{ label }}</span>
</template>
