const ALLOWED_PROTOCOL = /^(https?:|mailto:)/i;

const CODE_PLACEHOLDER = /\u0000(\d+)\u0000/g;

export function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

/** 只放行 http/https/mailto，阻断 javascript: 与 data:。 */
export function safeUrl(url: string): string | null {
  const trimmed = url.trim();
  if (!trimmed || !ALLOWED_PROTOCOL.test(trimmed)) return null;
  return trimmed;
}

function renderInline(source: string): string {
  let html = escapeHtml(source);

  // 行内代码先抽出来，避免内部内容被后续规则改写。
  const codes: string[] = [];
  html = html.replace(/`([^`]+)`/g, (_match, code: string) => {
    codes.push(code);
    return `\u0000${codes.length - 1}\u0000`;
  });

  html = html.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_match, label: string, url: string) => {
    const safe = safeUrl(url);
    if (!safe) return label;
    return `<a href="${safe}" target="_blank" rel="noreferrer noopener">${label}</a>`;
  });

  html = html.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  html = html.replace(/(^|[^*])\*([^*\n]+)\*/g, "$1<em>$2</em>");

  html = html.replace(CODE_PLACEHOLDER, (_match, index: string) => {
    return `<code>${codes[Number(index)] ?? ""}</code>`;
  });

  return html;
}

/**
 * 极简 Markdown 渲染：标题、段落、列表、引用、围栏代码、行内样式。
 * 所有文本先做 HTML 转义，链接限定 http/https/mailto。
 */
export function renderMarkdown(source: string): string {
  if (!source) return "";

  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const output: string[] = [];
  let index = 0;

  while (index < lines.length) {
    const line = lines[index];

    const fence = /^```(\w*)/.exec(line);
    if (fence) {
      const language = fence[1];
      index += 1;
      const code: string[] = [];
      while (index < lines.length && !/^```/.test(lines[index])) {
        code.push(lines[index]);
        index += 1;
      }
      index += 1;
      const className = language ? ` class="language-${escapeHtml(language)}"` : "";
      output.push(`<pre><code${className}>${escapeHtml(code.join("\n"))}</code></pre>`);
      continue;
    }

    if (line.trim() === "") {
      index += 1;
      continue;
    }

    const heading = /^(#{1,6})\s+(.*)$/.exec(line);
    if (heading) {
      const level = heading[1].length;
      output.push(`<h${level}>${renderInline(heading[2])}</h${level}>`);
      index += 1;
      continue;
    }

    if (/^\s*[-*+]\s+/.test(line)) {
      const items: string[] = [];
      while (index < lines.length && /^\s*[-*+]\s+/.test(lines[index])) {
        items.push(`<li>${renderInline(lines[index].replace(/^\s*[-*+]\s+/, ""))}</li>`);
        index += 1;
      }
      output.push(`<ul>${items.join("")}</ul>`);
      continue;
    }

    if (/^>\s?/.test(line)) {
      const quotes: string[] = [];
      while (index < lines.length && /^>\s?/.test(lines[index])) {
        quotes.push(renderInline(lines[index].replace(/^>\s?/, "")));
        index += 1;
      }
      output.push(`<blockquote>${quotes.join("<br>")}</blockquote>`);
      continue;
    }

    const paragraph: string[] = [];
    while (
      index < lines.length &&
      lines[index].trim() !== "" &&
      !/^```/.test(lines[index]) &&
      !/^(#{1,6})\s+/.test(lines[index]) &&
      !/^\s*[-*+]\s+/.test(lines[index]) &&
      !/^>\s?/.test(lines[index])
    ) {
      paragraph.push(renderInline(lines[index]));
      index += 1;
    }
    output.push(`<p>${paragraph.join("<br>")}</p>`);
  }

  return output.join("\n");
}
