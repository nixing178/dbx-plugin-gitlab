<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t } from "../i18n";
import { formatDateTime } from "../lib/format";
import { renderMarkdown } from "../lib/markdown";
import CommentComposer from "./CommentComposer.vue";
import ConfirmAction from "./ConfirmAction.vue";
import MarkdownBlock from "./MarkdownBlock.vue";
import StatusBadge from "./StatusBadge.vue";
import type { Issue, Note } from "../types";

const props = withDefaults(
  defineProps<{ projectPath: string; iid: number; readOnly?: boolean }>(),
  { readOnly: true },
);
const emit = defineEmits<{ back: [] }>();

const issue = ref<Issue | null>(null);
const notes = ref<Note[]>([]);
const loading = ref(true);
const error = ref("");
const actionError = ref("");
const busy = ref(false);
const commentBusy = ref(false);
const confirmKind = ref<"" | "close" | "reopen">("");
const editing = ref(false);
const editTitle = ref("");
const editDescription = ref("");

const assignees = computed(() =>
  (issue.value?.assignees ?? [])
    .map((user) => user.name || user.username)
    .filter(Boolean)
    .join(", "),
);

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const [detail, thread] = await Promise.all([
      gitlabApi.issue(props.projectPath, props.iid),
      gitlabApi.issueNotes(props.projectPath, props.iid).catch(() => [] as Note[]),
    ]);
    issue.value = detail;
    notes.value = thread;
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

async function toggleState(): Promise<void> {
  if (!issue.value) return;
  busy.value = true;
  actionError.value = "";
  try {
    await gitlabApi.updateIssue({
      projectId: props.projectPath,
      iid: props.iid,
      stateEvent: issue.value.state === "opened" ? "close" : "reopen",
    });
    confirmKind.value = "";
    await load();
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    busy.value = false;
  }
}

async function submitComment(body: string): Promise<void> {
  commentBusy.value = true;
  actionError.value = "";
  try {
    await gitlabApi.commentIssue(props.projectPath, props.iid, body);
    notes.value = await gitlabApi.issueNotes(props.projectPath, props.iid);
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    commentBusy.value = false;
  }
}

function startEdit(): void {
  editTitle.value = issue.value?.title ?? "";
  editDescription.value = issue.value?.description ?? "";
  editing.value = true;
}

async function saveEdit(): Promise<void> {
  busy.value = true;
  actionError.value = "";
  try {
    await gitlabApi.updateIssue({
      projectId: props.projectPath,
      iid: props.iid,
      title: editTitle.value.trim(),
      description: editDescription.value,
    });
    editing.value = false;
    await load();
  } catch (err) {
    actionError.value = describeError(err);
  } finally {
    busy.value = false;
  }
}

onMounted(() => void load());
</script>

<template>
  <section class="page">
    <div class="toolbar">
      <button class="btn ghost" type="button" @click="emit('back')">← {{ t("back") }}</button>
      <button class="btn" type="button" :disabled="loading" @click="load">
        {{ t("refresh") }}
      </button>
      <span class="grow"></span>
      <span v-if="issue" class="muted small mono">#{{ issue.iid }}</span>
    </div>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="load">{{ t("retry") }}</button>
    </div>
    <div v-if="actionError" class="error-banner">
      <span class="grow">{{ actionError }}</span>
      <button class="btn ghost" type="button" @click="actionError = ''">×</button>
    </div>

    <div v-if="loading && !issue" class="loading">{{ t("loading") }}</div>

    <div v-else-if="issue" class="detail-layout">
      <div class="detail-main">
        <div class="card card-body">
          <h1 class="issue-title">
            <span class="mono muted">#{{ issue.iid }}</span>
            {{ issue.title }}
          </h1>
          <div class="mr-meta">
            <StatusBadge :status="issue.state" />
            <span>
              {{ t("createdBy") }}:
              {{ issue.author?.name || issue.author?.username || "-" }}
            </span>
            <span>{{ formatDateTime(issue.created_at, locale) }}</span>
            <span v-if="issue.task_completion_status.count">
              · {{ issue.task_completion_status.completed_count }}/{{
                issue.task_completion_status.count
              }}
            </span>
          </div>
        </div>

        <div v-if="editing" class="card card-body">
          <div class="field">
            <label>{{ t("issueTitle") }}</label>
            <input v-model="editTitle" class="input" type="text" />
          </div>
          <div class="field">
            <label>{{ t("issueDescription") }}</label>
            <textarea v-model="editDescription" class="textarea" rows="6"></textarea>
          </div>
          <div class="composer-actions">
            <button class="btn" type="button" :disabled="busy" @click="editing = false">
              {{ t("cancel") }}
            </button>
            <button class="btn primary" type="button" :disabled="busy" @click="saveEdit">
              {{ busy ? t("loading") : t("save") }}
            </button>
          </div>
        </div>

        <div class="card card-body">
          <p v-if="!issue.description" class="empty">{{ t("noDescription") }}</p>
          <MarkdownBlock v-else :source="issue.description" />
        </div>

        <div class="card">
          <div class="card-head">
            <h2>{{ t("timeline") }}</h2>
            <span class="count">{{ notes.length }}</span>
          </div>

          <div class="card-body">
            <CommentComposer v-if="!readOnly" :busy="commentBusy" @submit="submitComment" />
            <p v-else class="muted small">{{ t("readOnlyActionHint") }}</p>
          </div>

          <div v-if="!notes.length" class="empty">{{ t("discussionEmpty") }}</div>
          <div v-else class="timeline">
            <div v-for="note in notes" :key="note.id" class="note">
              <div class="avatar">
                {{ (note.author?.name || note.author?.username || "?").slice(0, 1).toUpperCase() }}
              </div>
              <div class="note-main">
                <div class="note-head">
                  <strong>{{ note.author?.name || note.author?.username || "unknown" }}</strong>
                  <span>{{ formatDateTime(note.created_at, locale) }}</span>
                  <span v-if="note.system" class="badge" data-tone="neutral">
                    {{ t("systemNote") }}
                  </span>
                </div>
                <div class="note-body markdown" v-html="renderMarkdown(note.body)"></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <aside class="detail-side">
        <div class="card">
          <div class="card-head"><h2>{{ t("details") }}</h2></div>
          <div class="side-body">
            <div class="side-row">
              <span class="side-label">{{ t("createdBy") }}</span>
              <span class="small">{{ issue.author?.name || issue.author?.username || "-" }}</span>
            </div>
            <div class="side-row">
              <span class="side-label">{{ t("assignedTo") }}</span>
              <span class="small">{{ assignees || "-" }}</span>
            </div>
            <div class="side-row">
              <span class="side-label">{{ t("labelsField") }}</span>
              <span v-if="issue.labels.length" class="label-list">
                <span v-for="label in issue.labels" :key="label" class="badge" data-tone="info">
                  {{ label }}
                </span>
              </span>
              <span v-else class="small muted">-</span>
            </div>
            <div class="side-row">
              <span class="side-label">{{ t("milestone") }}</span>
              <span class="small">{{ issue.milestone?.title || "-" }}</span>
            </div>
            <div class="side-row">
              <span class="side-label">{{ t("issueComments") }}</span>
              <span class="small">{{ issue.user_notes_count }}</span>
            </div>
            <div class="side-row">
              <span class="side-label">{{ t("committedAt") }}</span>
              <span class="small">{{ formatDateTime(issue.updated_at, locale) }}</span>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="card-head"><h2>{{ t("actions") }}</h2></div>
          <div class="side-actions">
            <button
              v-if="issue.state === 'opened'"
              class="btn"
              type="button"
              :disabled="readOnly || busy"
              :title="readOnly ? t('readOnlyActionHint') : ''"
              @click="confirmKind = 'close'"
            >
              {{ t("closeIssue") }}
            </button>
            <button
              v-else
              class="btn"
              type="button"
              :disabled="readOnly || busy"
              :title="readOnly ? t('readOnlyActionHint') : ''"
              @click="confirmKind = 'reopen'"
            >
              {{ t("reopenIssue") }}
            </button>
            <button
              class="btn"
              type="button"
              :disabled="readOnly || busy"
              :title="readOnly ? t('readOnlyActionHint') : ''"
              @click="startEdit"
            >
              {{ t("edit") }}
            </button>
          </div>
        </div>
      </aside>
    </div>

    <ConfirmAction
      v-if="confirmKind"
      :title="confirmKind === 'close' ? t('closeIssue') : t('reopenIssue')"
      :confirm-label="confirmKind === 'close' ? t('closeIssue') : t('reopenIssue')"
      :busy="busy"
      :danger="confirmKind === 'close'"
      @confirm="toggleState"
      @cancel="confirmKind = ''"
    />
  </section>
</template>
