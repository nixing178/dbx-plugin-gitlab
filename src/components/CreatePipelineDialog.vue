<script setup lang="ts">
import { ref } from "vue";
import { describeError, gitlabApi } from "../api";
import { t } from "../i18n";
import type { GitLabProject, Pipeline, PipelineVariable } from "../types";

const props = defineProps<{ project: GitLabProject }>();
const emit = defineEmits<{ close: []; created: [pipeline: Pipeline] }>();

const refName = ref(props.project.default_branch || "main");
const variables = ref<PipelineVariable[]>([]);
const busy = ref(false);
const error = ref("");

function addVariable(): void {
  variables.value = [...variables.value, { key: "", value: "" }];
}

function removeVariable(index: number): void {
  variables.value = variables.value.filter((_, position) => position !== index);
}

async function submit(): Promise<void> {
  busy.value = true;
  error.value = "";
  try {
    const cleaned = variables.value.filter((variable) => variable.key.trim());
    const pipeline = await gitlabApi.createPipeline(
      props.project.path_with_namespace,
      refName.value.trim(),
      cleaned,
    );
    emit("created", pipeline);
  } catch (err) {
    error.value = describeError(err);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="overlay" @click.self="!busy && emit('close')">
    <section class="modal" role="dialog" aria-modal="true">
      <header class="modal-head">
        <h2>{{ t("createPipelineTitle") }}</h2>
        <button class="btn ghost" type="button" :disabled="busy" @click="emit('close')">×</button>
      </header>

      <div class="modal-body">
        <p v-if="error" class="error-text">{{ error }}</p>

        <div class="field">
          <label>{{ t("pipelineRef") }}</label>
          <input v-model="refName" class="input mono" type="text" placeholder="main" />
        </div>

        <div class="field">
          <label>{{ t("variables") }}</label>
          <div v-for="(variable, index) in variables" :key="index" class="variable-row">
            <input v-model="variable.key" class="input mono" type="text" :placeholder="t('variableKey')" />
            <input v-model="variable.value" class="input mono" type="text" :placeholder="t('variableValue')" />
            <button class="btn ghost small" type="button" @click="removeVariable(index)">×</button>
          </div>
          <button class="btn ghost small" type="button" @click="addVariable">
            + {{ t("addVariable") }}
          </button>
        </div>
      </div>

      <footer class="modal-foot">
        <button class="btn" type="button" :disabled="busy" @click="emit('close')">
          {{ t("cancel") }}
        </button>
        <button
          class="btn primary"
          type="button"
          :disabled="busy || !refName.trim()"
          @click="submit"
        >
          {{ busy ? t("loading") : t("runPipeline") }}
        </button>
      </footer>
    </section>
  </div>
</template>
