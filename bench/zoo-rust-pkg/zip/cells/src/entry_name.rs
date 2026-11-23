// @cell(F/pkg/zip/entry-name)
// @shape: the name of an entry of a zip archive is written by whoever made the archive, so name(), name_raw(), mangled_name() and file_names() are sources of zip (a skeleton of it): the entry point of zip slip. enclosed_name() is the crate's sanitized variant and is not a source. A name that went through the path sanitizer is not a finding, and a field that is not a name (the size) is not a source
use zip::read::ZipFile;
use zip::ZipArchive;
use zoo_mark as m;

pub fn register() {
    m::case("F/pkg/zip/entry-name/vuln", vuln);
    m::case("F/pkg/zip/entry-name/fixed", fixed);
    m::case("F/pkg/zip/entry-name/decoy", decoy);
}

// (the names are made with In::new, which is not a source of the rules: the source of the cell is the call of the package)
fn file(tag: &str) -> ZipFile {
    ZipFile::zoo(m::In::new(tag).value)
}

fn vuln() { // @vuln
    {
        let f = file("pkg-zip-entry-name-vuln-name");
        m::sink_path("/srv/base", &f.name().unwrap()); // @source(pkg-zip-entry-name-vuln-name) @sink(pkg-zip-entry-name-vuln-name)
    }
    {
        let f = file("pkg-zip-entry-name-vuln-raw");
        m::sink_path("/srv/base", &String::from_utf8_lossy(f.name_raw())); // @source(pkg-zip-entry-name-vuln-raw) @sink(pkg-zip-entry-name-vuln-raw)
    }
    {
        let f = file("pkg-zip-entry-name-vuln-mangled");
        m::sink_path("/srv/base", &f.mangled_name().to_string_lossy()); // @source(pkg-zip-entry-name-vuln-mangled) @sink(pkg-zip-entry-name-vuln-mangled)
    }
    {
        let a = ZipArchive::zoo(m::In::new("pkg-zip-entry-name-vuln-names").value);
        for n in a.file_names() { // @source(pkg-zip-entry-name-vuln-names)
            m::sink_path("/srv/base", &n.unwrap()); // @sink(pkg-zip-entry-name-vuln-names)
        }
    }
}

fn fixed() { // @fixed
    let f = file("pkg-zip-entry-name-fixed");
    m::sink_path("/srv/base", &m::clean_path(&f.name().unwrap())); // @sink(none)
    m::sink_path("/srv/base", &m::clean_path(&f.mangled_name().to_string_lossy())); // @sink(none)
}

fn decoy() { // @decoy
    let f = file("pkg-zip-entry-name-decoy");
    let v = f.name().unwrap().to_string(); // @source(pkg-zip-entry-name-decoy)
    m::sink_path("/srv/base", "constant"); // @sink(none) @nosink(pkg-zip-entry-name-decoy)
    m::sink_path("/srv/base", &f.size().to_string()); // @sink(none) @nosink(pkg-zip-entry-name-decoy)
    m::sink_path("/srv/base", &v); // @sink(pkg-zip-entry-name-decoy)
}
