use std::net::TcpStream;
use std::process::{Child, Command};
use std::sync::Mutex;
use std::time::Duration;
#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

static SIDECAR: Mutex<Option<Child>> = Mutex::new(None);

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            #[cfg(mobile)]
            unsafe { ffi::StartSeshat(); }

            #[cfg(not(mobile))]
            {
                let ext = if cfg!(target_os = "windows") { ".exe" } else { "" };
                let target = std::env::var("TARGET").unwrap_or_default();
                let name_long = format!("seshat_server-{}{}", target, ext);
                let name_short = format!("seshat_server{}", ext);

                // 1) bundled: next to the main executable
                // 2) dev: project_root/build/
                let bin = std::env::current_exe().ok()
                    .and_then(|p| p.parent().map(|d| d.to_path_buf()))
                    .and_then(|exe_dir| {
                        let bundled = exe_dir.join(&name_short);
                        if bundled.exists() { return Some(bundled); }
                        let bundled_long = exe_dir.join(&name_long);
                        if bundled_long.exists() { return Some(bundled_long); }
                        None
                    })
                    .or_else(|| {
                        let dev = std::env::current_dir().ok()?
                            .parent()?.join("build").join(&name_long);
                        if dev.exists() { Some(dev) } else { None }
                    });
                if let Some(path) = bin {
                    let mut cmd = Command::new(&path);
                    cmd.env("SESHAT_SIDECAR", "1");
                    #[cfg(target_os = "windows")]
                    { cmd.creation_flags(0x08000000); } // CREATE_NO_WINDOW
                    if let Ok(child) = cmd.spawn() {
                        *SIDECAR.lock().unwrap() = Some(child);
                        for _ in 0..30 {
                            std::thread::sleep(Duration::from_millis(100));
                            if TcpStream::connect("127.0.0.1:12500").is_ok() { break; }
                        }
                    }
                }
            }

            setup_menu(app)?;
            Ok(())
        })
        .on_window_event(|_window, event| {
            if let tauri::WindowEvent::Destroyed = event {
                #[cfg(not(mobile))]
                if let Some(mut child) = SIDECAR.lock().unwrap().take() {
                    let _ = child.kill();
                }
                #[cfg(mobile)]
                unsafe { ffi::StopSeshat(); }
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

fn setup_menu(app: &tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    use tauri::menu::{MenuBuilder, SubmenuBuilder, PredefinedMenuItem};

    let app_menu = SubmenuBuilder::new(app, "Seshat Desktop")
        .item(&PredefinedMenuItem::about(app, Some("关于 Seshat Desktop"), None)?)
        .separator()
        .item(&PredefinedMenuItem::hide(app, Some("隐藏 Seshat Desktop"))?)
        .item(&PredefinedMenuItem::hide_others(app, Some("隐藏其他"))?)
        .item(&PredefinedMenuItem::show_all(app, Some("全部显示"))?)
        .separator()
        .item(&PredefinedMenuItem::quit(app, Some("退出 Seshat Desktop"))?)
        .build()?;

    let edit_menu = SubmenuBuilder::new(app, "编辑")
        .item(&PredefinedMenuItem::undo(app, Some("撤销"))?)
        .item(&PredefinedMenuItem::redo(app, Some("重做"))?)
        .separator()
        .item(&PredefinedMenuItem::cut(app, Some("剪切"))?)
        .item(&PredefinedMenuItem::copy(app, Some("复制"))?)
        .item(&PredefinedMenuItem::paste(app, Some("粘贴"))?)
        .item(&PredefinedMenuItem::select_all(app, Some("全选"))?)
        .build()?;

    let menu = MenuBuilder::new(app)
        .item(&app_menu)
        .item(&edit_menu)
        .build()?;

    app.set_menu(menu)?;
    Ok(())
}

#[cfg(mobile)]
mod ffi {
    extern "C" {
        pub fn StartSeshat() -> i32;
        pub fn StopSeshat();
    }
}
