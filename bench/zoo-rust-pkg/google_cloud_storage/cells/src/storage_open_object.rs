// @cell(F/pkg/google-cloud-storage/storage-open-object)
// @shape: a value of a source is the name of the object of Storage::open_object
use zoo_mark as m;
use google_cloud_storage::client::Storage;

pub fn register() {
    m::case("F/pkg/google-cloud-storage/storage-open-object/vuln", vuln);
    m::case("F/pkg/google-cloud-storage/storage-open-object/fixed", fixed);
    m::case("F/pkg/google-cloud-storage/storage-open-object/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-gcs-storage-open-object-vuln"); // @source(pkg-gcs-storage-open-object-vuln)
    Storage.open_object("bucket", &v); // @sink(pkg-gcs-storage-open-object-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_resource(&m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("pkg-gcs-storage-open-object-fixed"))))));
    Storage.open_object("bucket", &v); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-gcs-storage-open-object-decoy"); // @source(pkg-gcs-storage-open-object-decoy)
    Storage.open_object("bucket", "constant"); // @sink(none) @nosink(pkg-gcs-storage-open-object-decoy)
    m::sink_any(&t); // @sink(pkg-gcs-storage-open-object-decoy)
}
