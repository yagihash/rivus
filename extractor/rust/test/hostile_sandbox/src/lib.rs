/// what the build script found out about its surroundings
pub fn report() -> &'static str {
    env!("HOSTILE_REPORT")
}

/// something for the extractor to read, so that the extraction has a result
pub fn passes_through(s: &str) -> String {
    s.to_string()
}
