"""005 §8 的容器內組包與內容檢查，由 build_local.sh 呼叫。只用 Python 標準函式庫。"""
import hashlib
import json
import re
from pathlib import Path
import shutil
import struct
import sys
import zipfile
import zlib

DEFAULTS = {"hd": 2, "pgxp": True, "filter": "xbr", "widescreen": True, "texture_perspective": True}
FORBIDDEN = {".bin", ".cue", ".iso", ".img", ".chd", ".mcd", ".mcr", ".state"}


def check_file(name, size):
    if Path(name).suffix.lower() in FORBIDDEN or size == 512 * 1024:
        raise RuntimeError(f"疑似原版素材：{name} ({size})")


def check_tree(root):
    for p in root.rglob("*"):
        if p.is_symlink():
            # appimagetool 在來源 AppDir 建立標準圖示連結，只接受這個已知目標。
            if p != root / "AppDir/.DirIcon" or p.resolve() != (root / "AppDir/ps-golem.png").resolve():
                raise RuntimeError(f"stage 不接受符號連結：{p}")
        if p.is_file():
            check_file(p, p.stat().st_size)


def icon(path):
    # 自製的幾何圖示，無外部美術素材。
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    rows = bytearray()
    for y in range(64):
        rows.append(0)
        for x in range(64):
            lit = (12 <= x < 26 and 12 <= y < 52) or (26 <= x < 48 and (12 <= y < 23 or 30 <= y < 40)) or (38 <= x < 48 and 23 <= y < 30)
            rows.extend((92, 206, 182, 255) if lit else (27, 35, 57, 255))
    path.write_bytes(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 64, 64, 8, 6, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(rows)) + chunk(b"IEND", b""))


def prepare(ver, stage, out):
    source = Path("workplace") / f"stage-{ver}"
    stage.mkdir()
    if not out.is_dir():
        raise RuntimeError("缺少 build_dist 的乾淨封包目錄")
    app = stage / "AppDir"
    (app / "usr/bin").mkdir(parents=True)
    for cmd in ("psgolem", "psexe"):
        shutil.copy2(source / "linux" / cmd, app / "usr/bin" / cmd)
    (app / "AppRun").write_text('#!/bin/sh\nexec "$APPDIR/usr/bin/psgolem" "$@"\n')
    (app / "AppRun").chmod(0o755)
    (app / "ps-golem.desktop").write_text("[Desktop Entry]\nType=Application\nName=ps-golem\nExec=psgolem\nIcon=ps-golem\nCategories=Game;Emulator;\nTerminal=false\n")
    icon(app / "ps-golem.png")
    shutil.copy2("LICENSE", app / "LICENSE")
    shutil.copy2("frontend/ebiten/ui-font-OFL.txt", app / "UI-FONT-LICENSE.txt")
    shutil.copy2("workplace/appimage/type2-runtime-LICENSE", app / "APPIMAGE-LICENSE.txt")
    for platform in ("linux", "windows", "macos"):
        root = stage / platform / "ps-golem"
        root.mkdir(parents=True)
        for name in ("LICENSE", "README.md"):
            shutil.copy2(name, root / name)
        shutil.copy2("frontend/ebiten/ui-font-OFL.txt", root / "UI-FONT-LICENSE.txt")
        (root / "docs").mkdir()
        shutil.copy2("docs/play.md", root / "docs/play.md")
        (root / "settings.json").write_text(json.dumps(DEFAULTS, indent=2) + "\n")
        for directory, text in (("bios", "把自己的 512 KiB PlayStation BIOS 放在這裡。只留一份供自動尋找。"), ("disc", "把自己的 cue 與所有 bin 放在這裡。只留一份 cue 供自動尋找。")):
            (root / directory).mkdir()
            # Windows 說明檔帶 BOM 與 CRLF；其他平台亦可讀。
            (root / directory / "放這裡.txt").write_bytes(b"\xef\xbb\xbf" + (text + "\r\n").encode("utf-8"))
        if platform == "linux":
            for name in ("psgolem", "psexe"):
                shutil.copy2(source / platform / name, root / name)
        elif platform == "windows":
            for name in ("psgolem.exe", "psexe.exe"):
                shutil.copy2(source / platform / name, root / name)
        elif platform == "macos":
            shutil.copytree(source / platform / "ps-golem.app", root / "ps-golem.app")
            shutil.copy2(source / platform / "psexe", root / "psexe")
    check_tree(stage)


def finish(ver, stage, out):
    image = out / f"ps-golem-{ver}-x86_64.AppImage"
    image.chmod(0o755)
    shutil.copy2(image, stage / "linux/ps-golem" / image.name)
    shutil.copy2("workplace/appimage/type2-runtime-LICENSE", stage / "linux/ps-golem/APPIMAGE-LICENSE.txt")
    check_tree(stage)
    inventory = {}
    for platform in ("linux", "windows", "macos"):
        base = stage / platform
        archive = out / f"ps-golem-{ver}-local-{platform}.zip"
        with zipfile.ZipFile(archive, "x", zipfile.ZIP_DEFLATED) as z:
            for p in sorted(base.rglob("*")):
                z.write(p, p.relative_to(base))
        with zipfile.ZipFile(archive) as z:
            if z.testzip() is not None:
                raise RuntimeError(f"ZIP CRC 失敗：{archive}")
            entries = []
            for i in z.infolist():
                if not i.filename.startswith("ps-golem/") or ".." in Path(i.filename).parts:
                    raise RuntimeError(f"ZIP 路徑錯誤：{i.filename}")
                check_file(i.filename, i.file_size)
                if not i.filename.isascii() and not i.flag_bits & 0x800:
                    raise RuntimeError(f"缺 UTF-8 旗標：{i.filename}")
                entries.append({"path": i.filename, "size": i.file_size, "mode": oct(i.external_attr >> 16)})
            assert json.loads(z.read("ps-golem/settings.json")) == DEFAULTS
            for folder in ("bios", "disc"):
                names = [i.filename for i in z.infolist() if i.filename.startswith(f"ps-golem/{folder}/") and not i.is_dir()]
                assert names == [f"ps-golem/{folder}/放這裡.txt"]
                assert z.read(names[0]).startswith(b"\xef\xbb\xbf")
            inventory[archive.name] = entries
    files = sorted(p for p in out.iterdir() if p.suffix in (".zip", ".AppImage") or p.name.endswith(".tar.gz"))
    sums, manifest = [], []
    for p in files:
        digest = hashlib.sha256(p.read_bytes()).hexdigest()
        sums.append(f"{digest}  {p.name}\n")
        manifest.append({"file": p.name, "size": p.stat().st_size, "sha256": digest})
    (out / "SHA256SUMS").write_text("".join(sums))
    (out / "manifest.json").write_text(json.dumps({"version": ver, "rights": "不含原版素材；本機驗收，未發布", "packages": manifest}, ensure_ascii=False, indent=2) + "\n")
    evidence = Path("workplace/local-pack") / ver
    evidence.mkdir(parents=True,exist_ok=True)
    (evidence / "packages.json").write_text(json.dumps(inventory, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(manifest, indent=2))
    print("三個 ZIP：CRC、頂層、預設設定、空素材目錄、UTF-8、素材排除通過")


def smoke(ver, stage, out):
    dest = Path("workplace/local-pack") / ver / "smoke"
    dest.mkdir()
    with zipfile.ZipFile(out / f"ps-golem-{ver}-local-linux.zip") as z:
        for i in z.infolist():
            assert i.filename.startswith("ps-golem/") and ".." not in Path(i.filename).parts
        z.extractall(dest)
    root = dest / "ps-golem"
    (root / f"ps-golem-{ver}-x86_64.AppImage").chmod(0o755)
    for cmd in ("psgolem", "psexe"):
        (root / cmd).chmod(0o755)
    # 提前用目前 UID 建立唯讀 bind mount 的檔案目標，避免 dockerd 代建。
    for name in ("bios/SCPH5500.BIN", "disc/sotn.cue", "disc/sotn.bin"):
        (root / name).touch()
    (dest / "extracted").mkdir()


def check_image(ver, stage, out):
    extracted = Path("workplace/local-pack") / ver / "smoke/extracted/squashfs-root"
    for p in extracted.rglob("*"):
        if p.is_symlink():
            assert p.name == ".DirIcon" and p.resolve() == (extracted / "ps-golem.png").resolve(), p
        if p.is_file():
            check_file(p, p.stat().st_size)
    source = stage / "AppDir"
    wanted = sorted(str(p.relative_to(source)) for p in source.rglob("*") if p.is_file())
    got = sorted(str(p.relative_to(extracted)) for p in extracted.rglob("*") if p.is_file())
    # appimagetool 會另外建立 .DirIcon。
    assert set(wanted) <= set(got) <= set(wanted) | {".DirIcon"}, (wanted, got)
    for name in wanted:
        if name.endswith(".desktop"):
            continue  # appimagetool 可追加 X-AppImage-* 欄位。
        assert (source / name).read_bytes() == (extracted / name).read_bytes(), name
    print("AppImage 實際 payload：檔案、執行檔逐位元比對與素材排除通過")


if __name__ == "__main__":
    action, version = sys.argv[1:]
    if not re.fullmatch(r"v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}",version):
        raise SystemExit("完整版號格式 v.X.Y.Z-YYYYMMDD")
    staging = Path("workplace") / f"stage-{version}-local"
    output = Path("dist-all") / version / "patch"
    {"prepare": prepare, "finish": finish, "smoke": smoke, "check-image": check_image}[action](version, staging, output)
