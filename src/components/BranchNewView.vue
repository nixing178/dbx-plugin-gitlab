<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { describeError, gitlabApi } from "../api";
import { t, type MessageKey } from "../i18n";
import { validateBranchName } from "../lib/branchName";
import type { GitLabProject } from "../types";
import GitRefSelect from "./GitRefSelect.vue";

const props = withDefaults(defineProps<{ project: GitLabProject; readOnly?: boolean }>(), {
  readOnly: true,
});
const emit = defineEmits<{ created: [name: string]; cancel: [] }>();

const name = ref("");
const refName = ref(props.project.default_branch || "main");
const busy = ref(false);
const error = ref("");
const nameInput = ref<HTMLInputElement | null>(null);

const nameError = computed(() => validateBranchName(name.value));
const nameErrorText = computed(() => {
  if (!name.value.trim()) return "";
  const key: MessageKey | null =
    nameError.value === "whitespace"
      ? "branchNameWhitespace"
      : nameError.value === "invalid"
        ? "branchNameInvalid"
        : null;
  return key ? t(key) : "";
});
const canSubmit = computed(
  () => !props.readOnly && !busy.value && nameError.value === null && refName.value.trim() !== "",
);

async function submit(): Promise<void> {
  if (!canSubmit.value) return;
  busy.value = true;
  error.value = "";
  try {
    const branch = await gitlabApi.createBranch(
      props.project.path_with_namespace,
      name.value.trim(),
      refName.value.trim(),
    );
    emit("created", branch.name);
  } catch (err) {
    error.value = describeError(err);
  } finally {
    busy.value = false;
  }
}

onMounted(() => nameInput.value?.focus());
</script>

<template>
  <section class="page gl-branch-new">
    <h1 class="gl-branch-new-title">{{ t("branchNew") }}</h1>

    <p v-if="readOnly" class="gl-inline-warn">{{ t("branchNewReadOnly") }}</p>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
    </div>

    <form class="card gl-branch-new-form" @submit.prevent="submit">
      <div class="card-body gl-new-mr-field">
        <label for="branch-new-name">{{ t("branchNewName") }}</label>
        <input
          id="branch-new-name"
          ref="nameInput"
          v-model="name"
          class="input mono"
          type="text"
          autocomplete="off"
          spellcheck="false"
          :placeholder="t('branchNewNamePlaceholder')"
          :disabled="readOnly || busy"
        />
        <p v-if="nameErrorText" class="error-text">{{ nameErrorText }}</p>
      </div>

      <div class="card-body gl-new-mr-field">
        <label>{{ t("branchNewFrom") }}</label>
        <GitRefSelect
          v-model="refName"
          :project-id="project.path_with_namespace"
          :clearable="false"
          :disabled="readOnly || busy"
        />
        <p class="hint">{{ t("branchNewFromHint") }}</p>
      </div>

      <div class="gl-branch-new-actions">
        <button class="btn primary" type="submit" :disabled="!canSubmit">
          {{ busy ? t("loading") : t("branchNewSubmit") }}
        </button>
        <button class="btn" type="button" :disabled="busy" @click="emit('cancel')">
          {{ t("cancel") }}
        </button>
      </div>
    </form>
  </section>
</template>

<style scoped>
.gl-branch-new-title {
  margin: 0;
  font-size: 20px;
}

.gl-branch-new-form {
  max-width: 660px;
  /* .card 默认 overflow:hidden 会把「创建自」下拉裁在卡片边缘。 */
  overflow: visible;
}

.gl-branch-new-form .card-body + .card-body {
  border-top: 1px solid var(--border);
}

.gl-branch-new-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 14px 14px;
}
</style>
