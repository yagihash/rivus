// @cell(F/dart/io-link-create-target)
// @shape: the target of a symbolic link (`std::os::unix::fs::symlink`) is a path sink of the built-in rows: a link whose target is a stored name points anywhere (zip slip by a link). The name of the link itself is the other argument. std has no recording skeleton, so the same value is also given to `sink_path` on the line: the run sees the sink there
use std::os::unix::fs::symlink;
use std::path::PathBuf;
use zoo_mark as m;

pub fn register() {
    m::case("F/dart/io-link-create-target/vuln", vuln);
    m::case("F/dart/io-link-create-target/fixed", fixed);
    m::case("F/dart/io-link-create-target/decoy", decoy);
}

fn scratch(name: &str) -> PathBuf {
    let dir = std::env::temp_dir().join(format!("zoo-link-{}-{}", std::process::id(), name));
    let _ = std::fs::create_dir_all(&dir);
    dir
}

fn vuln() { // @vuln
    let tmp = scratch("vuln");
    let a = m::src("dart-io-link-create-target-vuln"); // @source(dart-io-link-create-target-vuln)
    { m::sink_path("/srv/base", &a); let _ = symlink(&a, tmp.join("a")); } // @sink(dart-io-link-create-target-vuln)
    let _ = std::fs::remove_dir_all(&tmp);
}

fn fixed() { // @fixed
    let tmp = scratch("fixed");
    let a = m::clean_path(&m::src("dart-io-link-create-target-fixed"));
    { m::sink_path("/srv/base", &a); let _ = symlink(&a, tmp.join("a")); } // @sink(none)
    let _ = std::fs::remove_dir_all(&tmp);
}

fn decoy() { // @decoy
    let tmp = scratch("decoy");
    let v = m::src("dart-io-link-create-target-decoy"); // @source(dart-io-link-create-target-decoy)
    { m::sink_path("/srv/base", "constant"); let _ = symlink("constant", tmp.join("a")); } // @sink(none) @nosink(dart-io-link-create-target-decoy)
    m::sink_sql(&v); // @sink(dart-io-link-create-target-decoy)
    let _ = std::fs::remove_dir_all(&tmp);
}
