#[cfg(target_os = "linux")]
pub fn only_on_linux(s: &str) -> String {
    s.to_string()
}

#[cfg(target_os = "macos")]
pub fn only_on_macos(s: &str) -> String {
    s.to_string()
}

pub fn everywhere(s: &str) -> String {
    s.to_string()
}
