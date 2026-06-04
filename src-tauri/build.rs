use std::process::Command;

fn main() {
    tauri_build::build();

    let go_root = std::env::current_dir().unwrap().parent().unwrap().to_path_buf();

    let lib_name = if cfg!(target_os = "windows") {
        "seshat.lib"
    } else {
        "libseshat.a"
    };

    // Output to project-level build/
    let out = go_root.join("build").join(lib_name);
    std::fs::create_dir_all(out.parent().unwrap()).ok();

    let status = Command::new("go")
        .args([
            "build",
            "-tags", "cgo",
            "-buildmode=c-archive",
            "-o",
            out.to_str().unwrap(),
            ".",
        ])
        .current_dir(&go_root)
        .status()
        .expect("failed to run go build");

    if !status.success() {
        panic!("go build failed");
    }

    println!(
        "cargo:rustc-link-search=native={}",
        go_root.join("build").display()
    );
    println!("cargo:rustc-link-lib=static=seshat");
    if cfg!(target_os = "macos") {
        println!("cargo:rustc-link-lib=framework=CoreFoundation");
        println!("cargo:rustc-link-lib=framework=Security");
    }
}
