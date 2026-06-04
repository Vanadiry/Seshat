use std::process::Command;

fn main() {
    tauri_build::build();

    let go_root = std::env::current_dir().unwrap().parent().unwrap().to_path_buf();

    // Rebuild Go when frontend or Go source changes
    println!("cargo:rerun-if-changed={}", go_root.join("web").display());
    println!("cargo:rerun-if-changed={}", go_root.join("Core").display());
    println!("cargo:rerun-if-changed={}", go_root.join("main.go").display());
    println!("cargo:rerun-if-changed={}", go_root.join("ffi.go").display());
    println!("cargo:rerun-if-changed={}", go_root.join("embed.go").display());

    let out_dir = go_root.join("build");
    std::fs::create_dir_all(&out_dir).ok();

    let lib_name = if cfg!(target_os = "windows") {
        "seshat.lib"
    } else {
        "libseshat.a"
    };

    let goos = if cfg!(target_os = "linux") { "linux" }
               else if cfg!(target_os = "macos") { "darwin" }
               else { "windows" };

    let target = std::env::var("TARGET").unwrap_or_default();
    let goarch = if target.contains("aarch64") || target.contains("arm64") { "arm64" }
                 else { "amd64" };

    let out = out_dir.join(lib_name);
    let status = Command::new("go")
        .args([
            "build",
            "-tags", "ffi",
            "-buildmode=c-archive",
            "-o", out.to_str().unwrap(),
            ".",
        ])
        .env("GOOS", goos)
        .env("GOARCH", goarch)
        .current_dir(&go_root)
        .status()
        .expect("failed to run go build");

    if !status.success() {
        panic!("go build failed for {goos}/{goarch}");
    }

    println!("cargo:rustc-link-search=native={}", out_dir.display());
    println!("cargo:rustc-link-lib=static=seshat");
    if cfg!(target_os = "macos") {
        println!("cargo:rustc-link-lib=framework=CoreFoundation");
        println!("cargo:rustc-link-lib=framework=Security");
    }
}
