<script setup lang="ts">
import { ref } from "vue";
import { describeError, gitlabApi } from "../api";
import { t } from "../i18n";
import type { GitLabProject, Issue } from "../types";

const props = defineProps<{ project: GitLabProject }>();
const emit = defineEmits<{ close: []; created: [issue: Issue] }>();

const title = ref("");
const description = ref("");
const labels = ref("");
const busy = ref(false);
const error = ref("");

async function submit(): Promise<void> {
  busy.value = true;
  error.value = "";
  try {
    const parsedLabels = labels.value
      .split(",")
      .map((label) => label.trim())
      .filter(Boolean);
    const issue = await gitlabApi.createIssue({
      projectId: props.project.path_with_namespace,
      title: title.value.trim(),
      description: description.value,
      labels: parsedLabels,
    });
    emit("created", issue);
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
        <h2>{{ t("newIssue") }}</h2>
        <button class="btn ghost" type="button" :disabled="busy" @click="emit('close')">×</button>
      </header>

      <div class="modal-body">
        <p v-if="error" class="error-text">{{ error }}</p>

        <div class="field">
          <label>{{ t("issueTitle") }}</label>
          <input v-model="title" class="input" type="text" />
        </div>
        <div class="field">
          <label>{{ t("issueDescription") }}</label>
          <textarea v-model="description" class="textarea" rows="5"></textarea>
        </div>
        <div class="field">
          <label>{{ t("labelsField") }}（{{ t("labelsHint") }}）</label>
          <input v-model="labels" class="input mono" type="text" placeholder="bug, urgent" />
        </div>
      </div>

      <footer class="modal-foot">
        <button class="btn" type="button" :disabled="busy" @click="emit('close')">
          {{ t("cancel") }}
        </button>
        <button
          class="btn primary"
          type="button"
          :disabled="busy || !title.trim()"
          @click="submit"
        >
          {{ busy ? t("loading") : t("create") }}
        </button>
      </footer>
    </section>
  </div>
</template>
