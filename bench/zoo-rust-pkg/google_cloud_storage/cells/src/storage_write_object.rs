// @cell(F/pkg/google-cloud-storage/storage-write-object)
// @shape: a value of a source is the name of the bucket of Storage::write_object: the resource_name sink of the built-in rows
use zoo_mark as m;
use google_cloud_storage::client::Storage;

pub fn register() {
    m::case("F/pkg/google-cloud-storage/storage-write-object/vuln", vuln);
    m::case("F/pkg/google-cloud-storage/storage-write-object/fixed", fixed);
    m::case("F/pkg/google-cloud-storage/storage-write-object/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-gcs-storage-write-object-vuln"); // @source(pkg-gcs-storage-write-object-vuln)
    Storage.write_object(&v, "object", "payload"); // @sink(pkg-gcs-storage-write-object-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("pkg-gcs-storage-write-object-fixed")))));
    Storage.write_object(&v, "object", "payload"); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-gcs-storage-write-object-decoy"); // @source(pkg-gcs-storage-write-object-decoy)
    Storage.write_object("constant", "object", "payload"); // @sink(none) @nosink(pkg-gcs-storage-write-object-decoy)
    m::sink_any(&t); // @sink(pkg-gcs-storage-write-object-decoy)
}
