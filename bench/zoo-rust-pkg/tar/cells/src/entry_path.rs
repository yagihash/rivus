// @cell(F/pkg/tar/entry-path)
// @shape: the name of an entry of a tar archive (and the target of a link) is written by whoever made the archive, so it is a source of tar (a skeleton of it): the entry point of zip slip. A name that went through the path sanitizer is not a finding, and a field that is not a name (the size) is not a source
use tar::Entry;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/tar/entry-path/vuln", vuln);
    m::case("F/pkg/tar/entry-path/fixed", fixed);
    m::case("F/pkg/tar/entry-path/decoy", decoy);
}

// (the names are made with In::new, which is not a source of the rules: the source of the cell is the call of the package)
fn entry(tag: &str) -> Entry {
    Entry::zoo(m::In::new(tag).value, m::In::new(tag).value)
}

fn vuln() { // @vuln
    {
        let e = entry("pkg-tar-entry-path-vuln-path");
        m::sink_path("/srv/base", &e.path().unwrap().to_string_lossy()); // @source(pkg-tar-entry-path-vuln-path) @sink(pkg-tar-entry-path-vuln-path)
    }
    {
        let e = entry("pkg-tar-entry-path-vuln-header");
        m::sink_path("/srv/base", &e.header().path().unwrap().to_string_lossy()); // @source(pkg-tar-entry-path-vuln-header) @sink(pkg-tar-entry-path-vuln-header)
    }
    {
        let e = entry("pkg-tar-entry-path-vuln-link");
        m::sink_path("/srv/base", &e.link_name().unwrap().unwrap().to_string_lossy()); // @source(pkg-tar-entry-path-vuln-link) @sink(pkg-tar-entry-path-vuln-link)
    }
    {
        let e = entry("pkg-tar-entry-path-vuln-bytes");
        m::sink_path("/srv/base", &String::from_utf8_lossy(&e.path_bytes())); // @source(pkg-tar-entry-path-vuln-bytes) @sink(pkg-tar-entry-path-vuln-bytes)
    }
}

fn fixed() { // @fixed
    let e = entry("pkg-tar-entry-path-fixed");
    m::sink_path("/srv/base", &m::clean_path(&e.path().unwrap().to_string_lossy())); // @sink(none)
    m::sink_path("/srv/base", &m::clean_path(&e.header().link_name().unwrap().unwrap().to_string_lossy())); // @sink(none)
}

fn decoy() { // @decoy
    let e = entry("pkg-tar-entry-path-decoy");
    let v = e.path().unwrap().to_string_lossy().to_string(); // @source(pkg-tar-entry-path-decoy)
    m::sink_path("/srv/base", "constant"); // @sink(none) @nosink(pkg-tar-entry-path-decoy)
    m::sink_path("/srv/base", &e.header().size().unwrap().to_string()); // @sink(none) @nosink(pkg-tar-entry-path-decoy)
    m::sink_path("/srv/base", &v); // @sink(pkg-tar-entry-path-decoy)
}
