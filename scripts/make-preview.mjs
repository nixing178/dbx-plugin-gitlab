#!/usr/bin/env node
/**
 * 生成免安装预览页：把 ui/index.html 与 preview/mock.js 合成 preview/index.html。
 * 用法：pnpm build && node scripts/make-preview.mjs
 */
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const source = resolve(root, "ui/index.html");
const mock = resolve(root, "preview/mock.js");
const target = resolve(root, "preview/index.html");

const html = readFileSync(source, "utf8");
const bridge = readFileSync(mock, "utf8");

if (!html.includes("<head>")) {
  console.error("[make-preview] ui/index.html 缺少 <head>，先执行 pnpm build");
  process.exit(1);
}

const injected = html.replace("<head>", `<head>\n    <script>\n${bridge}\n    </script>`);

mkdirSync(dirname(target), { recursive: true });
writeFileSync(target, injected, "utf8");
console.log(`[make-preview] wrote preview/index.html (${(injected.length / 1024).toFixed(1)} kB)`);
