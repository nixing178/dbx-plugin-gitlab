#!/usr/bin/env python3
"""校验 dbxp 包结构与完整性（只用标准库）。

用法：
    python scripts/verify-package.py                 # 取 dist/ 下最新的 dbxp
    python scripts/verify-package.py path/to/x.dbxp

检查项：
1. 条目固定 6 个且顺序符合约定；
2. 包内 manifest 的 version 与根 manifest 一致，backend.executable 已被重写；
3. checksums.json 覆盖前 5 个文件，且 sha256 逐个匹配；
4. 同名 .artifact.json 的 size / sha256 与实际文件一致。
"""

from __future__ import annotations

import hashlib
import json
import sys
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DIST = ROOT / "dist"

EXPECTED_ORDER = [
    "assets/connection.svg",
    "assets/plugin.svg",
    "bin/windows-x64/dbx-plugin-gitlab.exe",
    "manifest.json",
    "ui/index.html",
    "checksums.json",
]
CHECKED_FILES = EXPECTED_ORDER[:5]


def pick_package() -> Path:
    if len(sys.argv) > 1:
        return Path(sys.argv[1]).resolve()
    candidates = sorted(DIST.glob("*.dbxp"), key=lambda p: p.stat().st_mtime)
    if not candidates:
        sys.exit(f"[verify-package] {DIST} 下没有 dbxp")
    return candidates[-1]


def main() -> int:
    package = pick_package()
    if not package.exists():
        sys.exit(f"[verify-package] 文件不存在：{package}")

    ok = True

    def fail(message: str) -> None:
        nonlocal ok
        ok = False
        print(f"  FAIL  {message}")

    print(f"[verify-package] {package.name} ({package.stat().st_size} bytes)")

    with zipfile.ZipFile(package) as archive:
        names = archive.namelist()
        print("\n[1] 条目与顺序")
        for name in EXPECTED_ORDER:
            if name not in names:
                fail(f"缺少条目 {name}")
        if names != EXPECTED_ORDER:
            fail(f"条目顺序/数量与约定不一致：{names}")
        else:
            print("  OK    6 个条目顺序正确")

        print("\n[2] 包内 manifest")
        manifest = json.loads(archive.read("manifest.json"))
        version = manifest.get("version")
        executable = manifest.get("entrypoints", {}).get("backend", {}).get("executable")
        print(f"  version            = {version}")
        print(f"  backend.executable = {executable}")
        if executable != "bin/windows-x64/dbx-plugin-gitlab.exe":
            fail("backend.executable 未被重写为 bin/windows-x64/dbx-plugin-gitlab.exe")

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
        for name in CHECKED_FILES:
            if name not in files:
                fail(f"checksums 未覆盖 {name}")
                continue
            actual = hashlib.sha256(archive.read(name)).hexdigest()
            if actual == files[name]:
                print(f"  OK    {name}")
            else:
                fail(f"{name} sha256 不匹配（包内 {actual}，记录 {files[name]}）")
        extra = [name for name in files if name not in CHECKED_FILES]
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
        print(f"  size  记录 {meta.get('size')} / 实际 {len(data)}")
        print(f"  sha256 记录 {meta.get('sha256')}")
        print(f"         实际 {digest}")
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
