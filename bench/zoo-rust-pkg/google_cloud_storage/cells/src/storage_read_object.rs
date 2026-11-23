// @cell(F/pkg/google-cloud-storage/storage-read-object)
// @shape: a value of a source is the name of the object of Storage::read_object of google-cloud-storage (a skeleton of it): the resource_name sink of the built-in rows
use zoo_mark as m;
use google_cloud_storage::client::Storage;

pub fn register() {
    m::case("F/pkg/google-cloud-storage/storage-read-object/vuln", vuln);
    m::case("F/pkg/google-cloud-storage/storage-read-object/fixed", fixed);
    m::case("F/pkg/google-cloud-storage/storage-read-object/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-gcs-storage-read-object-vuln"); // @source(pkg-gcs-storage-read-object-vuln)
    Storage.read_object("bucket", &v); // @sink(pkg-gcs-storage-read-object-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_resource(&m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("pkg-gcs-storage-read-object-fixed"))))));
    Storage.read_object("bucket", &v); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-gcs-storage-read-object-decoy"); // @source(pkg-gcs-storage-read-object-decoy)
    Storage.read_object("bucket", "constant"); // @sink(none) @nosink(pkg-gcs-storage-read-object-decoy)
    m::sink_any(&t); // @sink(pkg-gcs-storage-read-object-decoy)
}
