<script setup lang="ts">
import { ref } from "vue";
import { t } from "../i18n";

const props = withDefaults(
  defineProps<{
    placeholder?: string;
    busy?: boolean;
    submitLabel?: string;
    compact?: boolean;
  }>(),
  { placeholder: "", busy: false, submitLabel: "", compact: false },
);

const emit = defineEmits<{ submit: [body: string]; cancel: [] }>();

const body = ref("");

function submit(): void {
  const text = body.value.trim();
  if (!text || props.busy) return;
  emit("submit", text);
  body.value = "";
}
</script>

<template>
  <div class="composer">
    <textarea
      v-model="body"
      class="textarea"
      :rows="compact ? 2 : 3"
      :placeholder="placeholder || t('writeComment')"
      :disabled="busy"
      @keydown.ctrl.enter.prevent="submit"
    ></textarea>
    <div class="composer-actions">
      <button v-if="compact" class="btn ghost" type="button" :disabled="busy" @click="emit('cancel')">
        {{ t("cancel") }}
      </button>
      <button
        class="btn primary"
        type="button"
        :disabled="busy || !body.trim()"
        @click="submit"
      >
        {{ busy ? t("loading") : submitLabel || t("submitComment") }}
      </button>
    </div>
  </div>
</template>
