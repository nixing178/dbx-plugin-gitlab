<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t } from "../i18n";
import { relativeTime } from "../lib/format";
import CreateIssueDialog from "./CreateIssueDialog.vue";
import StatusBadge from "./StatusBadge.vue";
import type { IssueScope } from "../lib/nav";
import type { GitLabProject, Issue, Todo } from "../types";

const props = withDefaults(
  defineProps<{
    project: GitLabProject | null;
    readOnly?: boolean;
    scope?: IssueScope;
    state?: string;
    search?: string;
  }>(),
  { readOnly: true, scope: "todos", state: "opened", search: "" },
);
const emit = defineEmits<{
  open: [issue: Issue, projectPath: string];
  openTodo: [todo: Todo];
  "update:scope": [value: IssueScope];
  "update:state": [value: string];
  "update:search": [value: string];
}>();

type Scope = IssueScope;

const searchDraft = ref(props.search);
const issues = ref<Issue[]>([]);
const todos = ref<Todo[]>([]);
const page = ref(1);
const hasMore = ref(false);
const loading = ref(false);
const error = ref("");
const showCreate = ref(false);

function onStateChange(event: Event): void {
  emit("update:state", (event.target as HTMLSelectElement).value);
}

function submitSearch(): void {
  emit("update:search", searchDraft.value.trim());
}

const scopes = [
  { value: "todos", label: "myTodos" },
  { value: "assigned", label: "assignedToMe" },
  { value: "created", label: "createdByMe" },
  { value: "project", label: "currentProject" },
] as const;

const states = [
  { value: "opened", label: "stateOpened" },
  { value: "closed", label: "stateClosed" },
  { value: "all", label: "stateAll" },
] as const;

async function load(reset = true): Promise<void> {
  if (loading.value) return;
  loading.value = true;
  error.value = "";
  if (reset) page.value = 1;

  try {
    if (props.scope === "todos") {
      const result = await gitlabApi.todos("pending", page.value, 20);
      todos.value = reset ? result.items : [...todos.value, ...result.items];
      issues.value = [];
      hasMore.value = result.has_more;
    } else {
      const result = await gitlabApi.issues({
        projectId: props.scope === "project" ? props.project?.path_with_namespace ?? "" : "",
        scope:
          props.scope === "assigned"
            ? "assigned_to_me"
            : props.scope === "created"
              ? "created_by_me"
              : "",
        state: props.state === "all" ? "" : props.state,
        search: props.search,
        page: page.value,
        perPage: 20,
      });
      issues.value = reset ? result.items : [...issues.value, ...result.items];
      todos.value = [];
      hasMore.value = result.has_more;
    }
  } catch (err) {
    error.value = describeError(err);
  } finally {
    loading.value = false;
  }
}

function projectPathOf(issue: Issue): string {
  const reference = issue.references?.full ?? "";
  const index = reference.lastIndexOf("#");
  if (index > 0) return reference.slice(0, index);
  return props.project?.path_with_namespace ?? "";
}

function openIssue(issue: Issue): void {
  emit("open", issue, projectPathOf(issue));
}

async function markDone(todo: Todo): Promise<void> {
  error.value = "";
  try {
    await gitlabApi.markTodoDone(todo.id);
    todos.value = todos.value.filter((item) => item.id !== todo.id);
  } catch (err) {
    error.value = describeError(err);
  }
}

function onCreated(issue: Issue): void {
  showCreate.value = false;
  emit("open", issue, props.project?.path_with_namespace ?? "");
}

watch(
  () => [props.scope, props.state, props.search],
  () => void load(true),
);
watch(
  () => props.search,
  (value) => {
    searchDraft.value = value;
  },
);
onMounted(() => void load(true));
</script>

<template>
  <section class="page">
    <div class="toolbar">
      <div class="tabs">
        <button
          v-for="option in scopes"
          :key="option.value"
          type="button"
          :class="{ active: scope === option.value }"
          :disabled="option.value === 'project' && !project"
          @click="emit('update:scope', option.value)"
        >
          {{ t(option.label) }}
        </button>
      </div>
      <span class="grow"></span>
      <template v-if="scope !== 'todos'">
        <select :value="state" class="select" @change="onStateChange">
          <option v-for="option in states" :key="option.value" :value="option.value">
            {{ t(option.label) }}
          </option>
        </select>
        <input
          v-model="searchDraft"
          class="input"
          type="search"
          :placeholder="t('listSearchPlaceholder')"
          @keydown.enter="submitSearch"
        />
      </template>
      <button
        v-if="scope === 'project' && project"
        class="btn primary"
        type="button"
        :disabled="readOnly"
        :title="readOnly ? t('readOnlyActionHint') : ''"
        @click="showCreate = true"
      >
        {{ t("newIssue") }}
      </button>
      <button class="btn" type="button" :disabled="loading" @click="load(true)">
        {{ t("refresh") }}
      </button>
    </div>

    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="load(true)">{{ t("retry") }}</button>
    </div>

    <div class="card">
      <div v-if="loading && !issues.length && !todos.length" class="loading">
        {{ t("loading") }}
      </div>

      <div v-else-if="scope === 'todos'">
        <div v-if="!todos.length" class="empty">{{ t("noTodos") }}</div>
        <div v-else class="list">
          <div v-for="todo in todos" :key="todo.id" class="list-item">
            <div class="grow">
              <button
                class="title as-link"
                type="button"
                @click="emit('openTodo', todo)"
              >
                {{ todo.target?.title || todo.body || todo.target_url }}
              </button>
              <div class="sub">
                <span v-if="todo.project" class="mono">{{ todo.project.path_with_namespace }}</span>
                <span v-if="todo.target?.iid"> · #{{ todo.target.iid }}</span>
                · {{ relativeTime(todo.created_at, locale) }}
              </div>
            </div>
            <div class="right">
              <button
                class="btn ghost small"
                type="button"
                :disabled="readOnly"
                :title="readOnly ? t('readOnlyActionHint') : ''"
                @click="markDone(todo)"
              >
                {{ t("markDone") }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-else>
        <div v-if="!issues.length" class="empty">{{ t("noIssues") }}</div>
        <div v-else class="list">
          <button
            v-for="issue in issues"
            :key="issue.id"
            class="list-item"
            type="button"
            @click="openIssue(issue)"
          >
            <div>
              <div class="title">
                <span class="mono muted">#{{ issue.iid }}</span>
                {{ issue.title }}
              </div>
              <div class="sub">
                {{ issue.author?.name || issue.author?.username || "-" }}
                · {{ relativeTime(issue.updated_at, locale) }}
                <template v-if="issue.user_notes_count">
                  · {{ t("notesCount") }}: {{ issue.user_notes_count }}
                </template>
              </div>
              <div v-if="issue.labels.length" class="mr-meta" style="margin-top: 4px">
                <span v-for="label in issue.labels" :key="label" class="badge" data-tone="info">
                  {{ label }}
                </span>
              </div>
            </div>
            <div class="right">
              <StatusBadge :status="issue.state" />
            </div>
          </button>
        </div>
      </div>
    </div>

    <div v-if="hasMore" class="footer-note">
      <button class="btn" type="button" :disabled="loading" @click="page += 1; load(false)">
        {{ t("loadMore") }}
      </button>
    </div>

    <CreateIssueDialog
      v-if="showCreate && project"
      :project="project"
      @close="showCreate = false"
      @created="onCreated"
    />
  </section>
</template>
