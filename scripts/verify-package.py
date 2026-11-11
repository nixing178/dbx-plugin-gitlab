#!/usr/bin/env python3
"""校验 dbxp 包结构与完整性（只用标准库）。

用法：
    python scripts/verify-package.py                       # 取 dist/ 下最新的 dbxp
    python scripts/verify-package.py path/to/x.dbxp
    python scripts/verify-package.py path/to/x.dbxp --target darwin-arm64

target 解析顺序：`--target` 参数 > 包名后缀（`<id>-<version>-<target>.dbxp`）> 当前主机。
sidecar 的条目路径为 `bin/<target>/<binary>`，仅 windows target 带 `.exe` 后缀。

检查项：
1. 条目固定 6 个且顺序符合约定；
2. 包内 manifest 的 version 与根 manifest 一致，backend.executable 已按 target 重写；
3. checksums.json 覆盖前 5 个文件，且 sha256 逐个匹配；
4. 同名 .artifact.json 的 target / size / sha256 与实际文件一致。
"""

from __future__ import annotations

import hashlib
import json
import platform
import re
import sys
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DIST = ROOT / "dist"

TARGET_PATTERN = re.compile(r"^(?:windows|darwin|linux)-[a-z0-9]+$")
ARCH_ALIASES = {
    "amd64": "x64",
    "x86_64": "x64",
    "x64": "x64",
    "arm64": "arm64",
    "aarch64": "arm64",
}


def host_target() -> str:
    if sys.platform.startswith("win"):
        os_name = "windows"
    elif sys.platform == "darwin":
        os_name = "darwin"
    else:
        os_name = "linux"
    machine = platform.machine().lower()
    return f"{os_name}-{ARCH_ALIASES.get(machine, machine)}"


def target_from_package_name(package: Path) -> str | None:
    stem = package.name[: -len(".dbxp")] if package.name.endswith(".dbxp") else package.stem
    parts = stem.split("-")
    if len(parts) < 3:
        return None
    candidate = "-".join(parts[-2:])
    return candidate if TARGET_PATTERN.fullmatch(candidate) else None


def backend_binary() -> str:
    manifest = json.loads((ROOT / "manifest.json").read_text(encoding="utf-8"))
    executable = manifest.get("entrypoints", {}).get("backend", {}).get("executable") or ""
    name = executable.rsplit("/", 1)[-1]
    return name[:-4] if name.endswith(".exe") else name


def expected_order(target: str) -> list[str]:
    suffix = ".exe" if target.startswith("windows") else ""
    return [
        "assets/connection.svg",
        "assets/plugin.svg",
        f"bin/{target}/{backend_binary()}{suffix}",
        "manifest.json",
        "ui/index.html",
        "checksums.json",
    ]


def pick_package() -> Path:
    candidates = sorted(DIST.glob("*.dbxp"), key=lambda p: p.stat().st_mtime)
    if not candidates:
        sys.exit(f"[verify-package] {DIST} 下没有 dbxp")
    return candidates[-1]


def parse_arguments() -> tuple[Path, str | None]:
    package: Path | None = None
    target: str | None = None
    arguments = sys.argv[1:]
    index = 0
    while index < len(arguments):
        argument = arguments[index]
        if argument == "--target":
            index += 1
            if index >= len(arguments):
                sys.exit("[verify-package] --target 需要一个值")
            target = arguments[index]
        elif argument.startswith("--target="):
            target = argument.split("=", 1)[1]
        else:
            package = Path(argument)
        index += 1
    return (package.resolve() if package else pick_package()), target


def main() -> int:
    package, target_argument = parse_arguments()
    if not package.exists():
        sys.exit(f"[verify-package] 文件不存在：{package}")

    if not backend_binary():
        sys.exit("[verify-package] 根 manifest 缺少 entrypoints.backend.executable")

    target = target_argument or target_from_package_name(package) or host_target()
    expected = expected_order(target)
    checked_files = expected[:-1]

    ok = True

    def fail(message: str) -> None:
        nonlocal ok
        ok = False
        print(f"  FAIL  {message}")

    print(f"[verify-package] {package.name} ({package.stat().st_size} bytes)")
    print(f"[verify-package] target = {target}")

    with zipfile.ZipFile(package) as archive:
        names = archive.namelist()
        print("\n[1] 条目与顺序")
        for name in expected:
            if name not in names:
                fail(f"缺少条目 {name}")
        if names != expected:
            fail(f"条目顺序/数量与约定不一致：{names}")
        else:
            print("  OK    6 个条目顺序正确")

        print("\n[2] 包内 manifest")
        manifest = json.loads(archive.read("manifest.json"))
        version = manifest.get("version")
        executable = manifest.get("entrypoints", {}).get("backend", {}).get("executable")
        print(f"  version            = {version}")
        print(f"  backend.executable = {executable}")
        if executable != expected[2]:
            fail(f"backend.executable 未被重写为 {expected[2]}")

        root_manifest = json.loads((ROOT / "manifest.json").read_text(encoding="utf-8"))
        if root_manifest.get("version") != version:
            fail(f"根 manifest 版本 {root_manifest.get('version')} != 包内 {version}")
        else:
            print("  OK    与根 manifest 版本一致")
        if root_manifest.get("id") != manifest.get("id"):
            fail(f"插件 id 不一致：{root_manifest.get('id')} != {manifest.get('id')}")

        print("\n[3] checksums.json")
        checksums = json.loads(archive.read("checksums.json"))
        files = checksums.get("files", {})
        if checksums.get("algorithm") != "sha256":
            fail(f"algorithm = {checksums.get('algorithm')!r}，期望 'sha256'")
        for name in checked_files:
            if name not in files:
                fail(f"checksums 未覆盖 {name}")
                continue
            actual = hashlib.sha256(archive.read(name)).hexdigest()
            if actual == files[name]:
                print(f"  OK    {name}")
            else:
                fail(f"{name} sha256 不匹配（包内 {actual}，记录 {files[name]}）")
        extra = [name for name in files if name not in checked_files]
        if extra:
            fail(f"checksums 多出条目：{extra}")

    print("\n[4] artifact.json")
    artifact = package.with_name(package.name.replace(".dbxp", ".artifact.json"))
    if not artifact.exists():
        fail(f"缺少 {artifact.name}")
    else:
        meta = json.loads(artifact.read_text(encoding="utf-8"))
        data = package.read_bytes()
        digest = hashlib.sha256(data).hexdigest()
        print(f"  target 记录 {meta.get('target')} / 期望 {target}")
        print(f"  size   记录 {meta.get('size')} / 实际 {len(data)}")
        print(f"  sha256 记录 {meta.get('sha256')}")
        print(f"         实际 {digest}")
        if meta.get("target") != target:
            fail("artifact.target 与包名/参数推导的 target 不一致")
        if meta.get("size") != len(data):
            fail("artifact.size 与实际文件大小不一致")
        if meta.get("sha256") != digest:
            fail("artifact.sha256 与实际文件摘要不一致")
        if meta.get("url") and meta["url"] != package.name:
            fail(f"artifact.url {meta['url']!r} != {package.name!r}")

    print("\nRESULT:", "PASS" if ok else "FAIL")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
