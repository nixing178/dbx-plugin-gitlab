#!/usr/bin/env node
/**
 * 校验插件版本号在多处声明中保持一致。
 *
 * DBX 会用 manifest.version 与 sidecar 的 plugin/initialize 返回值比对，
 * 不一致会直接拒绝启动（"backend identity does not match manifest"）。
 * 版本号分散在 manifest.json、package.json、backend/main.go、client.go 的 User-Agent，
 * 打包前必须对齐。
 */
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");

function read(relative) {
  return readFileSync(resolve(root, relative), "utf8");
}

function fail(message) {
  console.error(`[check-version] ${message}`);
  process.exit(1);
}

const manifest = JSON.parse(read("manifest.json"));
const pkg = JSON.parse(read("package.json"));
const mainGo = read("backend/main.go");
const clientGo = read("backend/internal/gitlab/client.go");

const version = manifest.version;
if (!version) fail("manifest.json has no version");

const checks = [];

const mainVersion = mainGo.match(/Version:\s*"([^"]+)"/);
if (!mainVersion) fail("backend/main.go: cannot find Version field");
checks.push({ file: "backend/main.go", version: mainVersion[1] });

checks.push({ file: "package.json", version: pkg.version });

const userAgent = clientGo.match(/userAgent\s*=\s*"dbx-gitlab-plugin\/([^"]+)"/);
if (userAgent) {
  checks.push({ file: "backend/internal/gitlab/client.go (User-Agent)", version: userAgent[1] });
}

const mismatched = checks.filter((check) => check.version !== version);
if (mismatched.length > 0) {
  console.error(`[check-version] manifest.json = ${version}`);
  for (const check of mismatched) {
    console.error(`[check-version] ${check.file} = ${check.version}`);
  }
  console.error("[check-version] version mismatch; align all declarations before packaging");
  process.exit(1);
}

console.log(`[check-version] ok: ${version}`);
