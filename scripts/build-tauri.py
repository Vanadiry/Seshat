import subprocess
import os
import shutil
import glob
import platform


def get_version():
    return subprocess.check_output(["git", "describe", "--tags", "--abbrev=0"], text=True).strip()


def main():
    version = get_version()
    subprocess.run(["pnpm", "tauri", "build"], check=True)

    dst = "./build/desktop"
    if os.path.exists(dst):
        shutil.rmtree(dst)
    os.makedirs(dst, exist_ok=True)

    bundle_src = "./src-tauri/target/release/bundle"
    if not os.path.exists(bundle_src):
        print("WARN: bundle not found")
        return

    system = platform.system()
    arch = platform.machine()
    label = "macOS" if system == "Darwin" else system
    arch_label = "arm64" if arch == "arm64" else "amd64"

    patterns = {
        "Darwin":  "dmg/*.dmg",
        "Windows": "nsis/*.exe",
        "Linux":   "appimage/*.AppImage",
    }

    if system not in patterns:
        print(f"WARN: unsupported platform {system}")
        return

    pat = patterns[system]
    matches = glob.glob(f"{bundle_src}/{pat}")
    if matches:
        ext = matches[0].rsplit(".", 1)[-1]
        target = f"{dst}/Seshat-Desktop-{version}-{label}-{arch_label}.{ext}"
        shutil.copy(matches[0], target)
        print(f"OK. {target}")
    else:
        print("WARN: bundle not found")


if __name__ == "__main__":
    main()
