<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t } from "../i18n";
import { relativeTime } from "../lib/format";
import Icon from "./Icon.vue";
import type { Commit, GitLabProject } from "../types";

const props = defineProps<{ project: GitLabProject; refName?: string }>();
const emit = defineEmits<{ open: [commit: Commit] }>();

const commits = ref<Commit[]>([]);
const page = ref(1);
const hasMore = ref(false);
const loading = ref(false);
const error = ref("");

async function load(reset = true): Promise<void> {
  // 换分支时允许打断进行中的请求；「加载更多」才需要防重入。
  if (loading.value && !reset) return;
  loading.value = true;
  error.value = "";
  if (reset) page.value = 1;
  try {
    const result = await gitlabApi.commits(
      props.project.path_with_namespace,
      props.refName || props.project.default_branch,
      page.value,
      30,
    );
    commits.value = reset ? result.items : [...commits.value, ...result.items];
    hasMore.value = result.has_more;
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

/** 切换分支只改列表数据源，不重置当前详情页（详情页由路由承载）。 */
watch(
  () => [props.project.path_with_namespace, props.refName],
  () => void load(true),
);

onMounted(() => void load(true));
</script>

<template>
  <div class="card">
    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="load(true)">{{ t("retry") }}</button>
    </div>

    <div v-if="loading && !commits.length" class="loading">{{ t("loading") }}</div>
    <div v-else-if="!commits.length" class="empty">{{ t("noData") }}</div>
    <div v-else class="list">
      <button
        v-for="commit in commits"
        :key="commit.id"
        class="list-item commit-row"
        type="button"
        @click="emit('open', commit)"
      >
        <div class="grow">
          <div class="title">{{ commit.title }}</div>
          <div class="sub">
            <span class="mono commit-id">{{ commit.short_id }}</span>
            · {{ commit.author_name }}
            · {{ relativeTime(commit.committed_date, locale) }}
          </div>
        </div>
        <span class="commit-row-arrow" aria-hidden="true">
          <Icon name="chevron" :size="14" />
        </span>
      </button>
    </div>

    <div v-if="hasMore" class="footer-note">
      <button class="btn" type="button" :disabled="loading" @click="page += 1; load(false)">
        {{ t("loadMore") }}
      </button>
    </div>
  </div>
</template>
