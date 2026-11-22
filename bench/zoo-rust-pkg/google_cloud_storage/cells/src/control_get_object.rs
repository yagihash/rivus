// @cell(F/pkg/google-cloud-storage/control-get-object)
// @shape: a value of a source is the name of the object that a builder of StorageControl (get_object().set_object) gets: the resource_name sink of the built-in rows
use zoo_mark as m;
use google_cloud_storage::client::StorageControl;

pub fn register() {
    m::case("F/pkg/google-cloud-storage/control-get-object/vuln", vuln);
    m::case("F/pkg/google-cloud-storage/control-get-object/fixed", fixed);
    m::case("F/pkg/google-cloud-storage/control-get-object/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-gcs-control-get-object-vuln"); // @source(pkg-gcs-control-get-object-vuln)
    StorageControl.get_object().set_bucket("bucket").set_object(&v).send(); // @sink(pkg-gcs-control-get-object-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("pkg-gcs-control-get-object-fixed")))));
    StorageControl.get_object().set_bucket("bucket").set_object(&v).send(); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-gcs-control-get-object-decoy"); // @source(pkg-gcs-control-get-object-decoy)
    StorageControl.get_object().set_bucket("bucket").set_object("constant").send(); // @sink(none) @nosink(pkg-gcs-control-get-object-decoy)
    m::sink_any(&t); // @sink(pkg-gcs-control-get-object-decoy)
}
