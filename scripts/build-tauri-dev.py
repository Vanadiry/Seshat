import subprocess


def main():
    subprocess.run(["pnpm", "run", "build"], check=True)
    subprocess.run(["pnpm", "tauri", "dev"], check=True)


if __name__ == "__main__":
    main()
