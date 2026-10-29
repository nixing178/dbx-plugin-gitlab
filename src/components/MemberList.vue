<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t, type MessageKey } from "../i18n";
import { copyText } from "../lib/clipboard";
import { commitAvatarTone } from "../lib/commitList";
import { dateInputValue, formatDate } from "../lib/format";
import {
  accessLevelKey,
  canEditMemberRole,
  canManageMember,
  filterMembers,
  invitableRoles,
  isLastOwner,
  memberDisplayName,
  memberInitial,
  memberSourceKey,
  memberStateKey,
  PROJECT_MEMBER_LEVELS,
  sortMembers,
  type MemberSortKey,
  type MemberViewer,
} from "../lib/members";
import ConfirmAction from "./ConfirmAction.vue";
import EmptyState from "./EmptyState.vue";
import Icon from "./Icon.vue";
import MemberInviteDialog from "./MemberInviteDialog.vue";
import type { GitLabProject, GitLabUser, Member } from "../types";

const props = withDefaults(
  defineProps<{ project: GitLabProject; readOnly?: boolean; me?: GitLabUser | null }>(),
  { readOnly: false, me: null },
);

/** 角色下拉的等级顺序：高权限在前，和 GitLab 一致（不含群组的 Minimal Access）。 */
const ROLE_LEVELS = PROJECT_MEMBER_LEVELS;

const members = ref<Member[]>([]);
const hasMore = ref(false);
const search = ref("");
/** 0 表示全部角色，其余为 GitLab access_level。 */
const role = ref(0);
const sortKey = ref<MemberSortKey>("role");
const sortDesc = ref(true);
const loading = ref(false);
const error = ref("");
const toast = ref("");
/** 正在改角色的成员 id，用于禁用下拉。 */
const roleBusy = ref(0);
/** 正在改过期时间的成员 id。 */
const expiryBusy = ref(0);
const removeTarget = ref<Member | null>(null);
const removing = ref(false);

/** 筛选与排序都在本地做，页签里的数字始终是成员总数。 */
const visible = computed(() =>
  sortMembers(
    filterMembers(members.value, search.value, role.value),
    sortKey.value,
    sortDesc.value ? "desc" : "asc",
  ),
);

const inviteOpen = ref(false);
/** 当前账号在本项目的有效等级：0 表示未知或无管理权限。 */
const viewerLevel = ref(0);

const viewer = computed<MemberViewer>(() => ({
  id: props.me?.id ?? 0,
  accessLevel: viewerLevel.value,
}));

/** 邀请成员需要维护者及以上。 */
const canInvite = computed(() => !props.readOnly && viewerLevel.value >= 40);

const inviteTitle = computed(() => {
  if (props.readOnly) return t("readOnlyActionHint");
  return viewerLevel.value >= 40 ? t("memberInviteHint") : t("memberInviteNoPermission");
});

function roleText(level: number): string {
  return t(accessLevelKey(level) as MessageKey);
}

function stateLabel(member: Member): string {
  const key = memberStateKey(member.state);
  return key ? t(key as MessageKey) : "";
}

/** 来源未知时返回空串，由模板兜底显示 —。 */
function sourceLabel(member: Member): string {
  const key = memberSourceKey(member.source ?? "");
  return key ? t(key as MessageKey) : "";
}

function joinedLabel(member: Member): string {
  return formatDate(member.created_at ?? "", locale.value);
}

function expiryLabel(member: Member): string {
  return formatDate(member.expires_at ?? "", locale.value);
}

function avatarTone(member: Member): number {
  return commitAvatarTone(member.username || member.name);
}

/** 单成员接口失败时，退回到成员列表里找自己，避免整个页面变成只读。 */
function selfLevelFromList(): number {
  const username = props.me?.username;
  if (!username) return 0;
  return members.value.find((item) => item.username === username)?.access_level ?? 0;
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const [page, self] = await Promise.all([
      gitlabApi.projectMembers(props.project.path_with_namespace, "", 1, 100),
      props.me
        ? gitlabApi
            .projectMember(props.project.path_with_namespace, props.me.id)
            .catch(() => null)
        : Promise.resolve(null),
    ]);
    members.value = page.items;
    hasMore.value = page.has_more;
    viewerLevel.value = self?.access_level ?? selfLevelFromList();
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

/** 角色下拉选项：不高于自己等级的角色 + 目标当前角色。 */
function editableRoles(member: Member): number[] {
  const allowed = invitableRoles(viewer.value.accessLevel, ROLE_LEVELS);
  if (allowed.includes(member.access_level)) return allowed;
  return [...allowed, member.access_level].sort((a, b) => b - a);
}

/** 无权限操作的成员给出原因提示。 */
function roleHint(member: Member): string {
  if (props.me && member.id === props.me.id) return t("memberSelfRoleHint");
  if (member.source === "inherited") return t("memberRoleInheritedHint");
  if (!canManageMember(member, viewer.value)) return t("memberRoleNoPermission");
  if (isLastOwner(member, members.value)) return t("memberLastOwnerHint");
  return "";
}

/** 切换排序字段时回到该字段的常用方向：角色降序、姓名/过期时间升序。 */
function onSortChange(event: Event): void {
  sortKey.value = (event.target as HTMLSelectElement).value as MemberSortKey;
  sortDesc.value = sortKey.value === "role";
}

function flash(message: string): void {
  toast.value = message;
  window.setTimeout(() => {
    if (toast.value === message) toast.value = "";
  }, 1800);
}

async function copyUsername(member: Member): Promise<void> {
  if (await copyText(`@${member.username}`)) flash(t("copied"));
}

/** 改角色：失败时把下拉回滚到原值。 */
async function changeRole(member: Member, event: Event): Promise<void> {
  const select = event.target as HTMLSelectElement;
  const next = Number(select.value);
  const previous = member.access_level;
  if (!next || next === previous) return;
  if (!canEditMemberRole(member, props.readOnly, viewer.value)) return;
  if (isLastOwner(member, members.value)) {
    select.value = String(previous);
    return;
  }
  if (!editableRoles(member).includes(next)) {
    select.value = String(previous);
    return;
  }

  roleBusy.value = member.id;
  error.value = "";
  try {
    const updated = await gitlabApi.updateMemberRole(
      props.project.path_with_namespace,
      member.id,
      next,
    );
    member.access_level = updated.access_level || next;
    flash(t("memberRoleUpdated", { role: roleText(member.access_level) }));
  } catch (err) {
    select.value = String(previous);
    error.value = describeError(err);
  } finally {
    roleBusy.value = 0;
  }
}

/** 修改成员过期时间；清空日期即取消到期限制，失败时回滚输入框。 */
async function changeExpiry(member: Member, event: Event): Promise<void> {
  const input = event.target as HTMLInputElement;
  const next = input.value;
  const previous = member.expires_at ?? "";
  if (dateInputValue(previous) === next) return;
  if (props.readOnly || !canManageMember(member, viewer.value)) {
    input.value = dateInputValue(previous);
    return;
  }

  expiryBusy.value = member.id;
  error.value = "";
  try {
    const updated = await gitlabApi.updateMemberExpiry(
      props.project.path_with_namespace,
      member.id,
      member.access_level,
      next,
    );
    member.expires_at = updated.expires_at ?? "";
    flash(t("memberExpiryUpdated"));
  } catch (err) {
    input.value = dateInputValue(previous);
    error.value = describeError(err);
  } finally {
    expiryBusy.value = 0;
  }
}

function askRemove(member: Member): void {
  removeTarget.value = member;
}

async function onInvited(name: string): Promise<void> {
  inviteOpen.value = false;
  flash(t("memberInvited", { name }));
  await load();
}

async function confirmRemove(): Promise<void> {
  const member = removeTarget.value;
  if (!member) return;

  removing.value = true;
  error.value = "";
  try {
    await gitlabApi.removeMember(props.project.path_with_namespace, member.id);
    members.value = members.value.filter((item) => item.id !== member.id);
    removeTarget.value = null;
    flash(t("memberRemoved", { name: memberDisplayName(member) }));
  } catch (err) {
    error.value = describeError(err);
  } finally {
    removing.value = false;
  }
}

/** 菜单项点完后用冒泡关掉对应的 <details>。 */
function closeMenu(event: Event): void {
  const details = (event.currentTarget as HTMLElement | null)?.closest("details");
  if (details) details.open = false;
}

/** <details> 不处理外部点击，点空白时统一收起。 */
function onDocumentClick(event: MouseEvent): void {
  const target = event.target as HTMLElement | null;
  if (target?.closest(".gl-members .gl-menu")) return;
  document.querySelectorAll<HTMLDetailsElement>(".gl-members .gl-menu[open]").forEach((item) => {
    item.open = false;
  });
}

onMounted(() => {
  document.addEventListener("click", onDocumentClick);
  void load();
});

onBeforeUnmount(() => document.removeEventListener("click", onDocumentClick));
</script>

<template>
  <section class="gl-members">
    <header class="gl-members-head">
      <div class="gl-members-title">
        <h2>{{ t("projectMembers") }}</h2>
        <p>{{ t("membersIntro", { project: project.name }) }}</p>
      </div>
      <div class="gl-members-actions">
        <span v-if="toast" class="muted small">{{ toast }}</span>
        <button
          class="btn primary"
          type="button"
          :disabled="!canInvite"
          :title="inviteTitle"
          @click="inviteOpen = true"
        >
          {{ t("memberInvite") }}
        </button>
        <button
          class="gl-icon-btn"
          type="button"
          :disabled="loading"
          :title="t('refresh')"
          :aria-label="t('refresh')"
          @click="load"
        >
          <Icon name="refresh" :size="16" />
        </button>
      </div>
    </header>

    <nav class="project-tabs gl-members-tabs" role="navigation">
      <button class="active" type="button" aria-current="page">
        {{ t("members") }}<span class="gl-tab-count">{{ members.length }}</span>
      </button>
    </nav>

    <div class="gl-mr-filters">
      <label class="gl-scope-field">
        <Icon name="funnel" :size="14" />
        <select v-model="role" :aria-label="t('memberRole')">
          <option :value="0">{{ t("memberAllRoles") }}</option>
          <option v-for="level in ROLE_LEVELS" :key="level" :value="level">
            {{ roleText(level) }}
          </option>
        </select>
      </label>

      <label class="search-field gl-members-search">
        <Icon name="search" :size="15" />
        <input v-model="search" type="search" :placeholder="t('memberSearch')" />
      </label>

      <div class="gl-mr-sort">
        <select
          class="select"
          :value="sortKey"
          :aria-label="t('memberSortBy')"
          @change="onSortChange"
        >
          <option value="role">{{ t("memberRole") }}</option>
          <option value="name">{{ t("memberName") }}</option>
          <option value="expiry">{{ t("memberExpires") }}</option>
        </select>
        <button
          class="gl-icon-btn"
          type="button"
          :title="sortDesc ? t('mrSortDesc') : t('mrSortAsc')"
          :aria-label="sortDesc ? t('mrSortDesc') : t('mrSortAsc')"
          @click="sortDesc = !sortDesc"
        >
          <Icon name="sort" :size="15" :class="{ 'gl-sort-asc': !sortDesc }" />
        </button>
      </div>
    </div>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="load">{{ t("retry") }}</button>
    </div>

    <div class="card gl-members-card">
      <div v-if="loading && !members.length" class="loading">{{ t("loading") }}</div>
      <EmptyState
        v-else-if="!visible.length"
        :title="members.length ? t('memberFilterEmpty') : t('noMembers')"
        :hint="members.length ? '' : t('memberEmptyHint')"
      />
      <table v-else class="gl-members-table">
        <thead>
          <tr>
            <th class="gl-member-col-account">{{ t("memberAccount") }}</th>
            <th class="gl-member-col-source">{{ t("memberSource") }}</th>
            <th class="gl-member-col-role">{{ t("memberRole") }}</th>
            <th class="gl-member-col-expiry">{{ t("memberExpires") }}</th>
            <th class="gl-member-col-joined">{{ t("memberJoined") }}</th>
            <th class="gl-member-col-menu" :aria-label="t('actions')"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="member in visible" :key="member.id" class="gl-member-row">
            <td>
              <div class="gl-member-account">
                <span class="gl-member-avatar" :data-tone="avatarTone(member)">
                  {{ memberInitial(member) }}
                </span>
                <span class="gl-member-id">
                  <a
                    v-if="member.web_url"
                    class="gl-member-name"
                    :href="member.web_url"
                    target="_blank"
                    rel="noreferrer"
                    :title="t('openInGitLab')"
                  >
                    {{ memberDisplayName(member) }}
                  </a>
                  <span v-else class="gl-member-name">{{ memberDisplayName(member) }}</span>
                  <span class="gl-member-handle mono">@{{ member.username }}</span>
                </span>
                <span v-if="stateLabel(member)" class="gl-member-state">{{ stateLabel(member) }}</span>
              </div>
            </td>
            <td class="gl-member-source" :class="{ muted: !sourceLabel(member) }">
              {{ sourceLabel(member) || "—" }}
            </td>
            <td class="gl-member-role">
              <select
                v-if="canEditMemberRole(member, readOnly, viewer) && !isLastOwner(member, members)"
                class="select gl-member-role-select"
                :value="String(member.access_level)"
                :disabled="roleBusy === member.id"
                :aria-label="t('memberRole')"
                @change="changeRole(member, $event)"
              >
                <option
                  v-for="level in editableRoles(member)"
                  :key="level"
                  :value="String(level)"
                >
                  {{ roleText(level) }}
                </option>
              </select>
              <span v-else :title="roleHint(member)">
                {{ roleText(member.access_level) }}
              </span>
            </td>
            <td class="gl-member-expiry">
              <input
                v-if="!readOnly && canManageMember(member, viewer)"
                class="gl-member-expiry-input"
                type="date"
                :value="dateInputValue(member.expires_at ?? '')"
                :disabled="expiryBusy === member.id"
                :title="member.expires_at ? t('memberExpiryEditHint') : t('memberExpiryNever')"
                @change="changeExpiry(member, $event)"
              />
              <span v-else-if="member.expires_at" class="gl-member-expiry-date">
                <Icon name="calendar" :size="13" />
                {{ expiryLabel(member) }}
              </span>
              <span v-else class="muted">{{ t("memberNeverExpires") }}</span>
            </td>
            <td class="gl-member-joined" :class="{ muted: !joinedLabel(member) }">
              {{ joinedLabel(member) || "—" }}
            </td>
            <td class="gl-member-menu-cell">
              <details class="gl-menu">
                <summary class="gl-icon-btn" :aria-label="t('actions')" :title="t('actions')">
                  <Icon name="more" :size="16" />
                </summary>
                <div class="gl-menu-list" @click="closeMenu">
                  <button type="button" @click="copyUsername(member)">
                    {{ t("memberCopyUsername") }}
                  </button>
                  <a v-if="member.web_url" :href="member.web_url" target="_blank" rel="noreferrer">
                    {{ t("openInGitLab") }}
                  </a>
                  <button
                    v-if="canManageMember(member, viewer) && !isLastOwner(member, members)"
                    class="danger"
                    type="button"
                    :disabled="readOnly"
                    :title="readOnly ? t('readOnlyActionHint') : ''"
                    @click="askRemove(member)"
                  >
                    {{ t("memberRemove") }}
                  </button>
                </div>
              </details>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="hasMore" class="gl-members-more">{{ t("memberMoreHint") }}</div>
    </div>

    <MemberInviteDialog
      v-if="inviteOpen"
      :project-name="project.name"
      :project-id="project.path_with_namespace"
      :viewer-level="viewerLevel"
      @close="inviteOpen = false"
      @invited="onInvited"
    />

    <ConfirmAction
      v-if="removeTarget"
      :title="t('memberRemoveTitle')"
      :detail="t('memberRemoveDetail', { name: memberDisplayName(removeTarget) })"
      :confirm-label="t('memberRemove')"
      :busy="removing"
      @confirm="confirmRemove"
      @cancel="removeTarget = null"
    />
  </section>
</template>
