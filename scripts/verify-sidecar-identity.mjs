#!/usr/bin/env node
/**
 * 启动 sidecar 并校验 plugin/initialize 返回的身份与 manifest 一致。
 *
 * DBX 会拒绝 "backend identity does not match manifest" 的插件，
 * 打包后跑一遍可以避免装到本机才发现。
 *
 * 用法：node scripts/verify-sidecar-identity.mjs [dist/unpacked/bin/windows-x64/dbx-plugin-gitlab.exe]
 */
import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const manifest = JSON.parse(readFileSync(resolve("manifest.json"), "utf8"));
const exe = resolve(process.argv[2] ?? "dist/dbx-plugin-gitlab.exe");

const child = spawn(exe, [], { stdio: ["pipe", "pipe", "inherit"] });
let buffer = "";
let settled = false;

function finish(code) {
  if (settled) return;
  settled = true;
  clearTimeout(timer);
  child.kill();
  process.exit(code);
}

const timer = setTimeout(() => {
  console.error("[verify-sidecar] timeout waiting for plugin/initialize");
  finish(1);
}, 10000);

child.on("error", (error) => {
  console.error(`[verify-sidecar] cannot start ${exe}: ${error.message}`);
  finish(1);
});

child.on("exit", (code) => {
  if (!settled) {
    console.error(`[verify-sidecar] sidecar exited before responding (code ${code})`);
    finish(1);
  }
});

child.stdout.on("data", (data) => {
  buffer += data.toString("utf8");
  let index;
  while ((index = buffer.indexOf("\n")) >= 0) {
    const line = buffer.slice(0, index).trim();
    buffer = buffer.slice(index + 1);
    if (!line) continue;

    let message;
    try {
      message = JSON.parse(line);
    } catch {
      console.error(`[verify-sidecar] non-JSON output: ${line.slice(0, 200)}`);
      finish(1);
      return;
    }

    const plugin = message.result?.plugin;
    if (!plugin) {
      console.error(`[verify-sidecar] unexpected response: ${line.slice(0, 200)}`);
      finish(1);
      return;
    }
    if (plugin.id !== manifest.id) {
      console.error(
        `[verify-sidecar] id mismatch: backend=${plugin.id}, manifest=${manifest.id}`,
      );
      finish(1);
      return;
    }
    if (plugin.version !== manifest.version) {
      console.error(
        `[verify-sidecar] version mismatch: backend=${plugin.version}, manifest=${manifest.version}`,
      );
      finish(1);
      return;
    }
    console.log(`[verify-sidecar] ok: ${plugin.id}/${plugin.version}`);
    finish(0);
    return;
  }
});

child.stdin.write(
  `${JSON.stringify({
    jsonrpc: "2.0",
    id: 1,
    method: "plugin/initialize",
    params: {
      host: {
        dbxVersion: "0.6.47",
        hostApiVersion: "1.4.0",
        features: [],
        protocolVersions: [1],
      },
      plugin: { id: manifest.id, version: manifest.version },
      permissions: [],
    },
  })}\n`,
);
