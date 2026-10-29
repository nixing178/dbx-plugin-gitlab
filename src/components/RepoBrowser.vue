<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { describeError, gitlabApi } from "../api";
import { locale, t } from "../i18n";
import { copyText } from "../lib/clipboard";
import { formatBytes, formatDateTime, relativeTime } from "../lib/format";
import { parentPath } from "../lib/paths";
import Icon from "./Icon.vue";
import MarkdownBlock from "./MarkdownBlock.vue";
import type { Commit, FileContent, GitLabProject, TreeNode } from "../types";

const props = defineProps<{
  project: GitLabProject;
  refName: string;
  showReadme?: boolean;
}>();
const emit = defineEmits<{ "open-commit": [commit: Commit] }>();

const currentPath = ref("");
const items = ref<TreeNode[]>([]);
const commits = ref<Record<string, Commit | null>>({});
const page = ref(1);
const hasMore = ref(false);
const loading = ref(false);
const error = ref("");

const file = ref<FileContent | null>(null);
const fileLoading = ref(false);
const copied = ref(false);

const readme = ref("");
const readmeLoading = ref(false);
const readmeError = ref("");
let readmeLoaded = false;

const segments = computed(() => (currentPath.value ? currentPath.value.split("/") : []));

function lastCommit(node: TreeNode): Commit | null {
  return commits.value[node.path] ?? null;
}

function lastCommitTitle(node: TreeNode): string {
  return lastCommit(node)?.title ?? "";
}

function lastCommitTime(node: TreeNode): string {
  const commit = lastCommit(node);
  return commit ? relativeTime(commit.committed_date, locale.value) : "-";
}

/** 列宽有限，悬停时再给完整时间，避免日期被裁。 */
function lastCommitTimeTitle(node: TreeNode): string {
  const commit = lastCommit(node);
  return commit ? formatDateTime(commit.committed_date, locale.value) : "";
}

const CODE_EXTENSIONS = new Set([
  "js", "mjs", "cjs", "ts", "tsx", "jsx", "vue", "kt", "kts", "java", "groovy", "gradle",
  "go", "rs", "py", "rb", "php", "c", "h", "cc", "cpp", "hpp", "cs", "swift", "scala",
  "sh", "bash", "zsh", "ps1", "sql", "css", "scss", "less", "html", "htm", "lua", "dart",
]);
const CONFIG_EXTENSIONS = new Set([
  "json", "json5", "yml", "yaml", "toml", "ini", "conf", "cfg", "properties", "xml", "lock",
  "env", "editorconfig", "gitignore", "gitattributes", "gitmodules", "cmd", "bat", "plist",
]);
const IMAGE_EXTENSIONS = new Set(["png", "jpg", "jpeg", "gif", "svg", "webp", "ico", "bmp", "avif"]);
/** 没有扩展名但一眼能认出来的脚本/构建文件。 */
const CODE_FILE_NAMES = new Set(["dockerfile", "makefile", "jenkinsfile", "mvnw", "gradlew", "rakefile", "procfile"]);

type EntryKind = "tree" | "markdown" | "image" | "code" | "config" | "file";

function extensionOf(name: string): string {
  const dot = name.lastIndexOf(".");
  if (dot > 0) return name.slice(dot + 1).toLowerCase();
  return name.startsWith(".") ? name.slice(1).toLowerCase() : "";
}

/** 目录/文件类型只影响图标与配色，不参与数据逻辑。 */
function entryKind(node: TreeNode): EntryKind {
  if (node.type === "tree") return "tree";
  const name = node.name.toLowerCase();
  const ext = extensionOf(name);
  if (ext === "md" || ext === "markdown" || ext === "mdx" || ext === "rst") return "markdown";
  if (IMAGE_EXTENSIONS.has(ext)) return "image";
  if (CODE_EXTENSIONS.has(ext) || CODE_FILE_NAMES.has(name)) return "code";
  if (CONFIG_EXTENSIONS.has(ext)) return "config";
  return "file";
}

function entryIcon(node: TreeNode): string {
  switch (entryKind(node)) {
    case "tree":
      return "folder";
    case "markdown":
      return "file-text";
    case "image":
      return "file-image";
    case "code":
      return "file-code";
    default:
      return "file";
  }
}

/** 「最后提交」列也是提交入口，点标题直接进提交详情。 */
function openLastCommit(node: TreeNode): void {
  const commit = lastCommit(node);
  if (commit) emit("open-commit", commit);
}

async function loadCommits(nodes: TreeNode[]): Promise<void> {
  const targets = nodes.map((node) => node.path).filter((path) => !(path in commits.value));
  if (!targets.length) return;
  try {
    const rows = await gitlabApi.treeCommits(
      props.project.path_with_namespace,
      props.refName,
      targets,
    );
    const next = { ...commits.value };
    for (const row of rows) next[row.path] = row.commit;
    commits.value = next;
  } catch {
    // 最后提交列只是辅助信息，拉不到就不显示，不影响目录浏览。
  }
}

let dirToken = 0;

async function loadDir(reset = true): Promise<void> {
  const token = ++dirToken;
  loading.value = true;
  error.value = "";
  if (reset) page.value = 1;
  try {
    const result = await gitlabApi.tree(
      props.project.path_with_namespace,
      currentPath.value,
      props.refName,
      page.value,
      100,
    );
    if (token !== dirToken) return;
    items.value = reset ? result.items : [...items.value, ...result.items];
    hasMore.value = result.has_more;
    // 文件列表先出来，最后提交列随后补齐；60 个路径逐个查可能较慢。
    void loadCommits(result.items);
  } catch (err) {
    if (token !== dirToken) return;
    error.value = describeError(err);
  } finally {
    if (token === dirToken) loading.value = false;
  }
}

async function openDir(path: string): Promise<void> {
  if (currentPath.value === path && !file.value) return;
  currentPath.value = path;
  file.value = null;
  items.value = [];
  await loadDir(true);
}

function goUp(): void {
  void openDir(parentPath(currentPath.value));
}

async function openFile(node: TreeNode): Promise<void> {
  fileLoading.value = true;
  error.value = "";
  try {
    file.value = await gitlabApi.file(
      props.project.path_with_namespace,
      node.path,
      props.refName,
    );
  } catch (err) {
    error.value = describeError(err);
  } finally {
    fileLoading.value = false;
  }
}

async function copy(text: string): Promise<void> {
  copied.value = await copyText(text);
  if (copied.value) window.setTimeout(() => (copied.value = false), 1500);
}

async function loadReadme(): Promise<void> {
  if (readmeLoaded || !props.showReadme || readmeLoading.value) return;
  readmeLoading.value = true;
  readmeError.value = "";
  try {
    const tree = await gitlabApi.tree(
      props.project.path_with_namespace,
      "",
      props.refName,
      1,
      100,
    );
    const candidate = tree.items.find(
      (node) => node.type === "blob" && /^readme(\.(md|markdown|txt))?$/i.test(node.name),
    );
    if (!candidate) {
      readme.value = "";
      readmeLoaded = true;
      return;
    }
    const content = await gitlabApi.file(
      props.project.path_with_namespace,
      candidate.path,
      props.refName,
    );
    readme.value = content.too_large || content.binary ? "" : content.text;
    readmeLoaded = true;
  } catch (err) {
    readmeError.value = describeError(err);
  } finally {
    readmeLoading.value = false;
  }
}

watch(
  () => props.refName,
  () => {
    currentPath.value = "";
    file.value = null;
    items.value = [];
    commits.value = {};
    readme.value = "";
    readmeLoaded = false;
    void loadDir(true);
    void loadReadme();
  },
);

watch(
  () => props.showReadme,
  (show) => {
    if (show) void loadReadme();
  },
);

onMounted(() => {
  void loadDir(true);
  void loadReadme();
});
</script>

<template>
  <div class="gl-repo">
    <div v-if="error" class="error-banner">
      <span class="grow">{{ error }}</span>
      <button class="btn ghost" type="button" @click="loadDir(true)">{{ t("retry") }}</button>
    </div>

    <template v-if="file">
      <div class="gl-file-view">
        <div class="gl-file-view-head">
          <button class="gl-btn subtle" type="button" @click="file = null">
            <Icon name="chevron-left" :size="14" />
            {{ t("backToFiles") }}
          </button>
          <span class="mono grow gl-ellipsis">{{ file.path }}</span>
          <span class="muted small">{{ formatBytes(file.size) }}</span>
          <button class="gl-btn subtle" type="button" @click="copy(file.path)">
            {{ copied ? t("copied") : t("copyPath") }}
          </button>
        </div>
        <p v-if="fileLoading" class="loading">{{ t("loading") }}</p>
        <p v-else-if="file.too_large" class="empty">{{ t("fileTooLarge") }}</p>
        <p v-else-if="file.binary" class="empty">{{ t("binaryFile") }}</p>
        <pre v-else class="code-view">{{ file.text }}</pre>
      </div>
    </template>

    <template v-else>
      <div class="gl-file-card">
        <div class="gl-file-toolbar">
          <Icon class="gl-file-toolbar-icon" name="folder" :size="15" />
          <span class="gl-path-crumb">
            <button type="button" @click="openDir('')">{{ project.name }}</button>
            <template v-for="(segment, index) in segments" :key="index">
              <span class="crumb-sep">/</span>
              <button type="button" @click="openDir(segments.slice(0, index + 1).join('/'))">
                {{ segment }}
              </button>
            </template>
          </span>
          <span class="spacer"></span>
          <span v-if="items.length && !hasMore" class="gl-file-count">
            {{ t("fileCount", { count: items.length }) }}
          </span>
          <button
            v-if="currentPath"
            class="gl-icon-btn"
            type="button"
            :title="t('goUp')"
            :aria-label="t('goUp')"
            @click="goUp"
          >
            <Icon name="chevron-up" :size="14" />
          </button>
        </div>

        <div v-if="loading && !items.length" class="gl-file-skeleton" aria-hidden="true">
          <div v-for="row in 5" :key="row" class="gl-file-skeleton-row">
            <span class="gl-sk gl-sk-icon"></span>
            <span class="gl-sk gl-sk-name"></span>
            <span class="gl-sk gl-sk-commit"></span>
            <span class="gl-sk gl-sk-time"></span>
          </div>
        </div>
        <div v-else-if="!items.length" class="empty">{{ t("emptyDirectory") }}</div>
        <table v-else class="gl-file-table">
          <thead>
            <tr>
              <th>{{ t("fileName") }}</th>
              <th>{{ t("lastCommit") }}</th>
              <th class="gl-file-updated-col">{{ t("lastUpdated") }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="node in items" :key="node.path" class="gl-file-row">
              <td class="gl-file-name">
                <span class="gl-file-icon" :data-kind="entryKind(node)">
                  <Icon :name="entryIcon(node)" :size="16" />
                </span>
                <button
                  type="button"
                  :title="node.path"
                  @click="node.type === 'tree' ? openDir(node.path) : openFile(node)"
                >
                  {{ node.name }}
                </button>
              </td>
              <td class="gl-file-commit">
                <template v-if="lastCommit(node)">
                  <button
                    class="gl-file-commit-link"
                    type="button"
                    :title="lastCommitTitle(node)"
                    @click="openLastCommit(node)"
                  >
                    {{ lastCommitTitle(node) }}
                  </button>
                  <span class="gl-file-commit-meta">
                    <span class="mono">{{ lastCommit(node)?.short_id }}</span>
                    <span class="crumb-sep">·</span>
                    <span class="gl-ellipsis">{{ lastCommit(node)?.author_name }}</span>
                  </span>
                </template>
                <span v-else class="muted">-</span>
              </td>
              <td class="gl-file-updated">
                <span v-if="lastCommit(node)" :title="lastCommitTimeTitle(node)">{{ lastCommitTime(node) }}</span>
                <span v-else class="muted">-</span>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="hasMore" class="footer-note">
          <button class="btn" type="button" :disabled="loading" @click="page += 1; loadDir(false)">
            {{ t("loadMore") }}
          </button>
        </div>
      </div>

      <div v-if="showReadme && !currentPath" class="gl-readme card">
        <div class="gl-readme-head">
          <Icon name="file" :size="15" />
          <strong>README</strong>
        </div>
        <div v-if="readmeError" class="muted small">{{ readmeError }}</div>
        <div v-else-if="readmeLoading" class="loading">{{ t("loading") }}</div>
        <div v-else-if="!readme" class="empty">{{ t("readmeEmpty") }}</div>
        <MarkdownBlock v-else :source="readme" />
      </div>
    </template>
  </div>
</template>
