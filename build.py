import subprocess
import os

TARGETS = [
    ("darwin", "arm64"),
    ("darwin", "amd64"),
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("windows", "amd64"),
]

DIST = "build"

def get_version():
    try:
        tag = subprocess.run(
            ["git", "describe", "--tags", "--abbrev=0"],
            capture_output=True, text=True
        )
        return tag.stdout.strip()
    except Exception:
        return "dev"

def main():
    version = get_version()
    os.makedirs(DIST, exist_ok=True)
    for goos, goarch in TARGETS:
        ext = ".exe" if goos == "windows" else ""
        out = f"{DIST}/seshat-{version}-{goos}-{goarch}{ext}"
        env = {**os.environ, "GOOS": goos, "GOARCH": goarch}
        subprocess.run(["go", "build", "-o", out, "."], env=env, check=True)
        print(f"OK. {goos} {goarch}")

if __name__ == "__main__":
    main()
