<script setup lang="ts">
import { onMounted, ref } from "vue";
import { describeError, gitlabApi } from "../api";
import { t, type MessageKey } from "../i18n";
import { accessLevelKey } from "../lib/members";
import type { GitLabProject, Member } from "../types";

const props = defineProps<{ project: GitLabProject }>();

const members = ref<Member[]>([]);
const search = ref("");
const loading = ref(false);
const error = ref("");

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const page = await gitlabApi.projectMembers(
      props.project.path_with_namespace,
      search.value.trim(),
      1,
      100,
    );
    members.value = page.items;
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

function initial(member: Member): string {
  const source = member.name || member.username || "?";
  return source.slice(0, 1).toUpperCase();
}

function roleLabel(member: Member): string {
  return t(accessLevelKey(member.access_level) as MessageKey);
}

onMounted(() => void load());
</script>

<template>
  <div class="card">
    <div class="toolbar" style="padding: 12px 14px">
      <input
        v-model="search"
        class="input grow"
        type="search"
        :placeholder="t('memberSearch')"
        @keydown.enter="load"
      />
      <button class="btn" type="button" :disabled="loading" @click="load">
        {{ loading ? t("loading") : t("refresh") }}
      </button>
      <span class="muted small">{{ members.length }}</span>
    </div>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="load">{{ t("retry") }}</button>
    </div>

    <div v-if="loading && !members.length" class="loading">{{ t("loading") }}</div>
    <div v-else-if="!members.length" class="empty">{{ t("noMembers") }}</div>
    <div v-else class="list">
      <div v-for="member in members" :key="member.id" class="list-item">
        <div class="avatar">{{ initial(member) }}</div>
        <div class="grow">
          <div class="title">{{ member.name || member.username }}</div>
          <div class="sub">
            <span class="mono">@{{ member.username }}</span>
            <template v-if="member.state && member.state !== 'active'">
              · {{ member.state }}
            </template>
          </div>
        </div>
        <div class="right">
          <span class="badge" data-tone="info">{{ roleLabel(member) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
