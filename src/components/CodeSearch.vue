<script setup lang="ts">
import { ref } from "vue";
import { describeError, gitlabApi } from "../api";
import { t } from "../i18n";
import type { GitLabProject, SearchResult } from "../types";

const props = defineProps<{ project: GitLabProject }>();

const query = ref("");
const results = ref<SearchResult[]>([]);
const loading = ref(false);
const searched = ref(false);
const error = ref("");

async function run(): Promise<void> {
  const term = query.value.trim();
  if (!term) return;

  loading.value = true;
  error.value = "";
  try {
    const page = await gitlabApi.searchBlobs(
      props.project.path_with_namespace,
      term,
      1,
      30,
    );
    results.value = page.items;
    searched.value = true;
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="card">
    <div class="toolbar" style="padding: 12px 14px">
      <input
        v-model="query"
        class="input grow"
        type="search"
        :placeholder="t('searchCodePlaceholder')"
        @keydown.enter="run"
      />
      <button class="btn primary" type="button" :disabled="loading || !query.trim()" @click="run">
        {{ loading ? t("loading") : t("codeSearch") }}
      </button>
    </div>

    <p class="muted small" style="padding: 0 14px">{{ t("searchHint") }}</p>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="run">{{ t("retry") }}</button>
    </div>

    <div v-if="loading && !results.length" class="loading">{{ t("loading") }}</div>
    <div v-else-if="searched && !results.length" class="empty">{{ t("noSearchResults") }}</div>
    <div v-else class="list">
      <div v-for="item in results" :key="item.path + item.startline" class="list-item">
        <div class="grow">
          <div class="title mono">{{ item.path }}</div>
          <div class="sub">
            {{ t("matchAt") }} {{ item.startline }}
            <span class="muted"> · {{ item.ref }}</span>
          </div>
          <pre class="search-snippet">{{ item.data }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>
