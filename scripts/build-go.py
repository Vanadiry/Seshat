import subprocess
import os

TARGETS = [
    ("macOS",   "darwin",  "arm64", "arm64"),
    ("Linux",   "linux",   "amd64", "amd64"),
    ("Windows", "windows", "amd64", "amd64"),
]


def get_version():
    return subprocess.check_output(["git", "describe", "--tags", "--abbrev=0"], text=True).strip()


def build_frontend():
    subprocess.run(["pnpm", "run", "build"], check=True, cwd=".")


def main():
    version = get_version()
    out_dir = "./build/server"
    os.makedirs(out_dir, exist_ok=True)
    build_frontend()

    for label, goos, goarch, arch_label in TARGETS:
        ext = ".exe" if goos == "windows" else ""
        name = f"Seshat-Server-{version}-{label}-{arch_label}{ext}"
        env = {**os.environ, "GOOS": goos, "GOARCH": goarch}
        subprocess.run(["go", "build", "-ldflags=-s -w", "-o", f"{out_dir}/{name}", "."], env=env, check=True)
        print(f"OK. {name}")


if __name__ == "__main__":
    main()
