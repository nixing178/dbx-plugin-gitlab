import { describe, expect, it } from "vitest";
import { renderMarkdown, safeUrl } from "./markdown";

describe("renderMarkdown", () => {
  it("escapes raw html", () => {
    const html = renderMarkdown("<script>alert(1)</script>");
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
  });

  it("renders headings, bold and inline code", () => {
    const html = renderMarkdown("# Title\n\n**bold** and `code`");
    expect(html).toContain("<h1>Title</h1>");
    expect(html).toContain("<strong>bold</strong>");
    expect(html).toContain("<code>code</code>");
  });

  it("renders fenced code blocks without interpreting markdown", () => {
    const html = renderMarkdown("```go\nif a < b {\n}\n```");
    expect(html).toContain("<pre><code");
    expect(html).toContain("a &lt; b");
    expect(html).not.toContain("<strong>");
  });

  it("rejects javascript urls", () => {
    const html = renderMarkdown("[click](javascript:alert(1))");
    expect(html).not.toContain("javascript:");
    expect(html).toContain("click");
  });

  it("keeps https links with escaped ampersands", () => {
    expect(renderMarkdown("[site](https://example.com/a?b=1&c=2)")).toContain(
      'href="https://example.com/a?b=1&amp;c=2"',
    );
  });

  it("renders unordered lists", () => {
    const html = renderMarkdown("- one\n- two");
    expect(html).toContain("<li>one</li>");
    expect(html).toContain("<li>two</li>");
  });

  it("renders blockquotes", () => {
    expect(renderMarkdown("> quoted")).toContain("<blockquote>");
  });
});

describe("safeUrl", () => {
  it("allows http, https and mailto", () => {
    expect(safeUrl("https://example.com")).toBe("https://example.com");
    expect(safeUrl("http://example.com")).toBe("http://example.com");
    expect(safeUrl("mailto:user@example.com")).toBe("mailto:user@example.com");
  });

  it("rejects script and relative urls", () => {
    expect(safeUrl("javascript:alert(1)")).toBeNull();
    expect(safeUrl("data:text/html,<script>")).toBeNull();
    expect(safeUrl("/group/project")).toBeNull();
  });
});
