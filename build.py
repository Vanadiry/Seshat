import subprocess
import os
import platform
import urllib.request

TARGETS = [
    ("darwin", "arm64"),
    ("darwin", "amd64"),
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("windows", "amd64"),
]

DIST = "build"
TW_DIR = "Tailwind"
TW_VERSION = "v4.3.0"

def get_version():
    try:
        tag = subprocess.run(
            ["git", "describe", "--tags", "--abbrev=0"],
            capture_output=True, text=True
        )
        return tag.stdout.strip()
    except Exception:
        return "dev"

def ensure_tailwind():
    sys_os = platform.system()
    if sys_os == "Darwin":
        tw_os = "macos"
    elif sys_os == "Linux":
        tw_os = "linux"
    elif sys_os == "Windows":
        tw_os = "windows"
    else:
        raise RuntimeError(f"Unsupported OS: {sys_os}")

    machine = platform.machine().lower()
    if machine in ("arm64", "aarch64"):
        tw_arch = "arm64"
    elif machine in ("x86_64", "amd64"):
        tw_arch = "x64"
    else:
        raise RuntimeError(f"Unsupported arch: {machine}")

    ext = ".exe" if tw_os == "windows" else ""
    name = f"tailwindcss-{tw_os}-{tw_arch}{ext}"
    path = os.path.join(TW_DIR, name)
    if os.path.exists(path):
        return path

    url = f"https://github.com/tailwindlabs/tailwindcss/releases/download/{TW_VERSION}/{name}"
    print(f"Downloading {url} ...")
    urllib.request.urlretrieve(url, path)
    os.chmod(path, 0o755)
    return path

def main():
    version = get_version()
    os.makedirs(DIST, exist_ok=True)
    os.makedirs(TW_DIR, exist_ok=True)

    # Compile Tailwind CSS
    tw = ensure_tailwind()
    subprocess.run(
        [tw, "-i", f"{TW_DIR}/tailwind-input.css",
         "-o", "web/assets/style.css", "--minify"], check=True
    )

    for goos, goarch in TARGETS:
        ext = ".exe" if goos == "windows" else ""
        out = f"{DIST}/seshat-{version}-{goos}-{goarch}{ext}"
        env = {**os.environ, "GOOS": goos, "GOARCH": goarch}
        subprocess.run(["go", "build", "-o", out, "."], env=env, check=True)
        print(f"OK. {goos} {goarch}")

if __name__ == "__main__":
    main()
