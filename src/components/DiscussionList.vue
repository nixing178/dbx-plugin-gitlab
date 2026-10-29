<script setup lang="ts">
import { computed, ref } from "vue";
import { locale, t } from "../i18n";
import { formatDateTime } from "../lib/format";
import { renderMarkdown } from "../lib/markdown";
import CommentComposer from "./CommentComposer.vue";
import type { Discussion, GitLabUser } from "../types";

const props = withDefaults(
  defineProps<{ discussions: Discussion[]; readOnly?: boolean; hideSystem?: boolean }>(),
  { readOnly: true, hideSystem: false },
);

/** 「仅评论」筛选：把系统备注剔掉，整条线程只剩系统备注时也一并隐藏。 */
const visible = computed<Discussion[]>(() => {
  if (!props.hideSystem) return props.discussions;
  return props.discussions
    .map((discussion) => ({
      ...discussion,
      notes: discussion.notes.filter((note) => !note.system),
    }))
    .filter((discussion) => discussion.notes.length > 0);
});

const emit = defineEmits<{
  reply: [discussionId: string, body: string];
  resolve: [discussionId: string, resolved: boolean];
}>();

const replyTo = ref<string | null>(null);

function initial(user: GitLabUser | null): string {
  const name = user?.name || user?.username || "?";
  return name.slice(0, 1).toUpperCase();
}

function hasResolvable(discussion: Discussion): boolean {
  return discussion.notes.some((note) => note.resolvable);
}

function isResolved(discussion: Discussion): boolean {
  return discussion.notes.some((note) => note.resolved);
}

function submitReply(discussionId: string, body: string): void {
  emit("reply", discussionId, body);
  replyTo.value = null;
}
</script>

<template>
  <div v-if="!visible.length" class="empty">{{ t("discussionEmpty") }}</div>
  <div v-else class="timeline">
    <article v-for="discussion in visible" :key="discussion.id" class="discussion">
      <div v-for="note in discussion.notes" :key="note.id" class="note">
        <div class="avatar">{{ initial(note.author) }}</div>
        <div class="note-main">
          <div class="note-head">
            <strong>{{ note.author?.name || note.author?.username || "unknown" }}</strong>
            <span>{{ formatDateTime(note.created_at, locale) }}</span>
            <span v-if="note.system" class="badge" data-tone="neutral">{{ t("systemNote") }}</span>
            <span v-if="note.resolved" class="badge" data-tone="success">{{ t("resolved") }}</span>
            <span v-if="note.position" class="inline-anchor">
              {{ t("inlineComment") }} · {{ note.position.new_path }}:{{ note.position.new_line }}
            </span>
          </div>
          <div class="note-body markdown" v-html="renderMarkdown(note.body)"></div>
        </div>
      </div>

      <div class="discussion-actions">
        <button
          class="btn ghost small"
          type="button"
          @click="replyTo = replyTo === discussion.id ? null : discussion.id"
        >
          {{ t("reply") }}
        </button>
        <button
          v-if="!readOnly && hasResolvable(discussion)"
          class="btn ghost small"
          type="button"
          @click="emit('resolve', discussion.id, !isResolved(discussion))"
        >
          {{ isResolved(discussion) ? t("unresolve") : t("resolve") }}
        </button>
      </div>

      <CommentComposer
        v-if="replyTo === discussion.id"
        compact
        :submit-label="t('reply')"
        @submit="(body) => submitReply(discussion.id, body)"
        @cancel="replyTo = null"
      />
    </article>
  </div>
</template>
