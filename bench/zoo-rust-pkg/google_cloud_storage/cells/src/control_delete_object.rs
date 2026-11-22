// @cell(F/pkg/google-cloud-storage/control-delete-object)
// @shape: a value of a source is the name of the bucket that delete_object().set_bucket of StorageControl takes
use zoo_mark as m;
use google_cloud_storage::client::StorageControl;

pub fn register() {
    m::case("F/pkg/google-cloud-storage/control-delete-object/vuln", vuln);
    m::case("F/pkg/google-cloud-storage/control-delete-object/fixed", fixed);
    m::case("F/pkg/google-cloud-storage/control-delete-object/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-gcs-control-delete-object-vuln"); // @source(pkg-gcs-control-delete-object-vuln)
    StorageControl.delete_object().set_bucket(&v).set_object("object").send(); // @sink(pkg-gcs-control-delete-object-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("pkg-gcs-control-delete-object-fixed")))));
    StorageControl.delete_object().set_bucket(&v).set_object("object").send(); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-gcs-control-delete-object-decoy"); // @source(pkg-gcs-control-delete-object-decoy)
    StorageControl.delete_object().set_bucket("constant").set_object("object").send(); // @sink(none) @nosink(pkg-gcs-control-delete-object-decoy)
    m::sink_any(&t); // @sink(pkg-gcs-control-delete-object-decoy)
}
