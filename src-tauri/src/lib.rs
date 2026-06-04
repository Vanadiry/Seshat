#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .setup(|_app| {
            unsafe { ffi::StartSeshat(); }
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

mod ffi {
    extern "C" {
        pub fn StartSeshat() -> i32;
        pub fn StopSeshat();
    }
}
