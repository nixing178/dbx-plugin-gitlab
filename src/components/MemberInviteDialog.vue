<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { t, type MessageKey } from "../i18n";
import { accessLevelKey, invitableRoles, memberInviteTarget } from "../lib/members";
import type { GitLabUser } from "../types";

const props = defineProps<{
  projectName: string;
  projectId: string;
  /** 当前账号在项目里的等级，决定能分配哪些角色。 */
  viewerLevel: number;
}>();
const emit = defineEmits<{ close: []; invited: [name: string] }>();

/** 邀请可选角色：GitLab 邀请页不提供 Minimal，默认访客。 */
const ROLE_LEVELS = [10, 20, 30, 40, 50];

/** 可分配不高于自己等级的角色（含同级）。 */
const roleOptions = computed(() => invitableRoles(props.viewerLevel, ROLE_LEVELS));

const target = ref("");
const accessLevel = ref(10);
const expiresAt = ref("");

/** 角色选项随查看者等级变化，越界时回落到最低可用角色。 */
watch(
  roleOptions,
  (list) => {
    if (!list.includes(accessLevel.value)) accessLevel.value = list[0] ?? 0;
  },
  { immediate: true },
);
const suggestions = ref<GitLabUser[]>([]);
const suggestionsOpen = ref(false);
const busy = ref(false);
const error = ref("");
let searchTimer = 0;
/** 从联想里选中后回填输入框，不再用它触发一次搜索。 */
let skipSearch = false;

/** 输入 2 个字符以上才联想；邮箱输入不做联想。 */
watch(target, (value) => {
  window.clearTimeout(searchTimer);
  if (skipSearch) {
    skipSearch = false;
    return;
  }
  const needle = value.trim();
  if (needle.length < 2 || needle.includes("@")) {
    suggestions.value = [];
    suggestionsOpen.value = false;
    return;
  }
  searchTimer = window.setTimeout(() => void searchUsers(needle), 300);
});

async function searchUsers(needle: string): Promise<void> {
  try {
    const result = await gitlabApi.searchUsers(needle);
    if (target.value.trim() !== needle) return;
    suggestions.value = result.items;
    suggestionsOpen.value = result.items.length > 0;
  } catch {
    /* 联想失败时仍允许手输用户名 */
    suggestions.value = [];
    suggestionsOpen.value = false;
  }
}

function pick(user: GitLabUser): void {
  skipSearch = true;
  target.value = user.username;
  suggestions.value = [];
  suggestionsOpen.value = false;
}

function hideSuggestions(): void {
  suggestionsOpen.value = false;
}

async function submit(): Promise<void> {
  const parsed = memberInviteTarget(target.value);
  if (!parsed) {
    error.value = t("memberInviteTargetRequired");
    return;
  }

  busy.value = true;
  error.value = "";
  try {
    const member =
      parsed.kind === "email"
        ? await gitlabApi.inviteMember(props.projectId, parsed.value, accessLevel.value, expiresAt.value)
        : await gitlabApi.addMember(props.projectId, parsed.value, accessLevel.value, expiresAt.value);
    emit("invited", member.name || member.username || parsed.value);
  } catch (err) {
    error.value = describeError(err);
  } finally {
    busy.value = false;
  }
}

onBeforeUnmount(() => window.clearTimeout(searchTimer));
</script>

<template>
  <div class="overlay" @click.self="!busy && emit('close')">
    <section class="modal gl-invite" role="dialog" aria-modal="true">
      <header class="modal-head">
        <h2>{{ t("memberInvite") }}</h2>
        <button class="btn ghost" type="button" :disabled="busy" @click="emit('close')">×</button>
      </header>

      <div class="modal-body">
        <p class="gl-invite-intro">{{ t("memberInviteIntro", { project: projectName }) }}</p>

        <div class="field gl-invite-field">
          <label for="gl-invite-target">{{ t("memberInviteTarget") }}</label>
          <div class="gl-invite-input">
            <input
              id="gl-invite-target"
              v-model="target"
              class="input"
              type="text"
              autocomplete="off"
              :placeholder="t('memberInviteTargetPlaceholder')"
              @focus="suggestionsOpen = suggestions.length > 0"
              @blur="hideSuggestions"
              @keydown.enter="submit"
            />
            <ul v-if="suggestionsOpen" class="gl-invite-suggestions">
              <li v-for="user in suggestions" :key="user.id">
                <button type="button" @mousedown.prevent="pick(user)">
                  <span class="gl-invite-suggestion-name">{{ user.name || user.username }}</span>
                  <span class="muted small mono">@{{ user.username }}</span>
                </button>
              </li>
            </ul>
          </div>
          <p class="muted small gl-invite-hint">{{ t("memberInviteTargetHint") }}</p>
        </div>

        <div class="field">
          <label for="gl-invite-role">{{ t("memberInviteRole") }}</label>
          <select id="gl-invite-role" v-model.number="accessLevel" class="select gl-invite-select">
            <option v-for="level in roleOptions" :key="level" :value="level">
              {{ t(accessLevelKey(level) as MessageKey) }}
            </option>
          </select>
          <a
            class="gl-invite-link"
            href="https://docs.gitlab.com/ee/user/permissions.html"
            target="_blank"
            rel="noreferrer"
          >
            {{ t("memberInviteRoleHint") }}
          </a>
        </div>

        <div class="field">
          <label for="gl-invite-expires">{{ t("memberInviteExpires") }}</label>
          <input id="gl-invite-expires" v-model="expiresAt" class="input gl-invite-date" type="date" />
          <a
            class="gl-invite-link"
            href="https://docs.gitlab.com/ee/user/project/members/#member-access-expiration"
            target="_blank"
            rel="noreferrer"
          >
            {{ t("memberInviteExpiresHint") }}
          </a>
        </div>

        <div v-if="error" class="error-banner">
          <span class="grow">{{ error }}</span>
        </div>
      </div>

      <footer class="modal-foot">
        <button class="btn" type="button" :disabled="busy" @click="emit('close')">
          {{ t("cancel") }}
        </button>
        <button class="btn primary" type="button" :disabled="busy" @click="submit">
          {{ busy ? t("loading") : t("memberInviteSubmit") }}
        </button>
      </footer>
    </section>
  </div>
</template>
