#!/usr/bin/env node
/**
 * 校验 manifest 的 mcp.tools 声明与 sidecar 实际提供的工具一致。
 *
 * DBX 外部 MCP 走严格契约：mcp/tools 返回但未在 manifest.tools 声明的工具
 * 永远不会对外暴露；反之声明的工具若 sidecar 不再提供，授权列表也会失真。
 * 新增或删除工具时必须同步 manifest.json。
 */
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");

function read(relative) {
  return readFileSync(resolve(root, relative), "utf8");
}

function fail(message) {
  console.error(`[check-mcp-tools] ${message}`);
  process.exit(1);
}

const manifest = JSON.parse(read("manifest.json"));
const source = read("backend/internal/rpc/mcp.go");

const contributions = (manifest.contributions ?? []).filter((item) => item?.type === "mcp");
if (contributions.length !== 1) {
  fail(`expected exactly one mcp contribution, found ${contributions.length}`);
}

const mcp = contributions[0];
const declared = mcp.tools ?? {};
const toolNames = new Set(
  [...source.matchAll(/newTool\(\s*"([a-z0-9_]+)"/g)].map((match) => match[1]),
);

if (toolNames.size === 0) fail("no tools found in backend/internal/rpc/mcp.go");

const toolsetIds = new Set((mcp.toolsets ?? []).map((toolset) => toolset.id));
const accessLevels = new Set(["read", "write", "full"]);

for (const [name, declaration] of Object.entries(declared)) {
  if (!toolNames.has(name)) {
    fail(`manifest declares "${name}" but mcp/tools does not provide it`);
  }
  if (!accessLevels.has(declaration?.access)) {
    fail(`"${name}" has invalid access level: ${declaration?.access}`);
  }
  if (declaration.toolset && !toolsetIds.has(declaration.toolset)) {
    fail(`"${name}" references unknown toolset "${declaration.toolset}"`);
  }
}

const undeclared = [...toolNames].filter((name) => !(name in declared));
if (undeclared.length > 0) {
  fail(`tools missing from manifest.tools: ${undeclared.join(", ")}`);
}

const count = Object.keys(declared).length;
if (mcp.external_tools === true && count === 0) {
  fail("external_tools is true but no tools are declared; the external MCP server would expose nothing");
}
if (count > 64) fail(`too many tool declarations: ${count} (max 64)`);

const byAccess = { read: 0, write: 0, full: 0 };
for (const declaration of Object.values(declared)) byAccess[declaration.access] += 1;

console.log(
  `[check-mcp-tools] ok: ${count} tools (read ${byAccess.read} / write ${byAccess.write} / full ${byAccess.full}) across ${toolsetIds.size} toolsets`,
);
