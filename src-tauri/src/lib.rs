#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            unsafe { ffi::StartSeshat(); }
            setup_menu(app)?;
            Ok(())
        })
        .on_window_event(|_window, event| {
            if let tauri::WindowEvent::Destroyed = event {
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

mod ffi {
    extern "C" {
        pub fn StartSeshat() -> i32;
        pub fn StopSeshat();
    }
}
