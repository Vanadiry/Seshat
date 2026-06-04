import subprocess
import os
import sys
import shutil
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

def build_frontend():
    subprocess.run(["pnpm", "run", "build"], check=True, cwd=ROOT)

def get_version():
    conf = json.loads((ROOT / "src-tauri" / "tauri.conf.json").read_text())
    return conf["version"]

TARGETS = [
    ("macOS",   "darwin",  "arm64"),
    ("Linux",   "linux",   "amd64"),
    ("Windows", "windows", "amd64"),
]

def cmd_server():
    """Build Go standalone binaries → build/server/"""
    version = get_version()
    out_dir = ROOT / "build" / "server"
    out_dir.mkdir(parents=True, exist_ok=True)
    build_frontend()
    for label, goos, goarch in TARGETS:
        ext = ".exe" if goos == "windows" else ""
        name = f"Seshat-Server-{version}-{label}-{goarch}{ext}"
        out = out_dir / name
        env = {**os.environ, "GOOS": goos, "GOARCH": goarch}
        subprocess.run(["go", "build", "-ldflags=-s -w", "-o", str(out), "."], cwd=ROOT, env=env, check=True)
        print(f"OK. {name}")

def cmd_server_dev():
    """Build and run Go binary locally"""
    out_dir = ROOT / "build" / "server"
    out_dir.mkdir(parents=True, exist_ok=True)
    build_frontend()
    out = out_dir / "Seshat_dev"
    subprocess.run(["go", "build", "-o", str(out), "."], cwd=ROOT, check=True)
    print(f"OK. {out}")
    subprocess.run([str(out)], check=True)

def cmd_desktop():
    """Build Tauri Desktop bundle → build/desktop/"""
    import glob as _glob
    version = get_version()
    subprocess.run(["pnpm", "tauri", "build"], cwd=ROOT, check=True)
    dst = ROOT / "build" / "desktop"
    if dst.exists():
        shutil.rmtree(dst)
    dst.mkdir(parents=True, exist_ok=True)
    bundle = ROOT / "src-tauri" / "target" / "release" / "bundle"
    patterns = [("dmg/*.dmg", "macOS-arm64.dmg"), ("nsis/*.exe", "Windows-amd64.exe"), ("appimage/*.AppImage", "Linux-amd64.AppImage")]
    for pat, ext in patterns:
        matches = _glob.glob(str(bundle / pat))
        if matches:
            target = dst / f"Seshat-Desktop-{version}-{ext}"
            shutil.copy(matches[0], target)
            print(f"OK. {target}")

def cmd_desktop_dev():
    """Build frontend and run Tauri dev"""
    build_frontend()
    subprocess.run(["pnpm", "tauri", "dev"], cwd=ROOT, check=True)

def cmd_clean():
    """Remove all build artifacts"""
    for d in ["build", "src-tauri/target", "src-tauri/gen/schemas"]:
        p = ROOT / d
        if p.exists():
            shutil.rmtree(p)
            print(f"OK. removed {p}")
    print("Clean done.")

COMMANDS = {
    "server":     cmd_server,
    "server-dev": cmd_server_dev,
    "desktop":    cmd_desktop,
    "desktop-dev": cmd_desktop_dev,
    "clean":      cmd_clean,
}

def main():
    if len(sys.argv) < 2 or sys.argv[1] not in COMMANDS:
        print("Usage: python3 scripts/build.py <command>")
        print("Commands: " + ", ".join(COMMANDS.keys()))
        sys.exit(1)
    COMMANDS[sys.argv[1]]()

if __name__ == "__main__":
    main()
