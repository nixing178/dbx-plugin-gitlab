<script setup lang="ts">
import { t } from "../i18n";

withDefaults(
  defineProps<{
    title: string;
    detail?: string;
    confirmLabel?: string;
    busy?: boolean;
    danger?: boolean;
  }>(),
  { detail: "", confirmLabel: "", busy: false, danger: true },
);

const emit = defineEmits<{ confirm: []; cancel: [] }>();
</script>

<template>
  <div class="overlay" @click.self="!busy && emit('cancel')">
    <section class="modal" role="dialog" aria-modal="true">
      <header class="modal-head">
        <h2>{{ title }}</h2>
        <button class="btn ghost" type="button" :disabled="busy" @click="emit('cancel')">×</button>
      </header>
      <div class="modal-body">
        <p v-if="detail" class="muted small">{{ detail }}</p>
        <slot />
      </div>
      <footer class="modal-foot">
        <button class="btn" type="button" :disabled="busy" @click="emit('cancel')">
          {{ t("cancel") }}
        </button>
        <button
          class="btn"
          :class="danger ? 'danger' : 'primary'"
          type="button"
          :disabled="busy"
          @click="emit('confirm')"
        >
          {{ busy ? t("loading") : confirmLabel || t("confirm") }}
        </button>
      </footer>
    </section>
  </div>
</template>
