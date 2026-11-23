// @cell(F/pkg/google-cloud-storage/control-list-objects)
// @shape: a value of a source is the parent (the bucket) of list_objects().set_parent of StorageControl
use zoo_mark as m;
use google_cloud_storage::client::StorageControl;

pub fn register() {
    m::case("F/pkg/google-cloud-storage/control-list-objects/vuln", vuln);
    m::case("F/pkg/google-cloud-storage/control-list-objects/fixed", fixed);
    m::case("F/pkg/google-cloud-storage/control-list-objects/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-gcs-control-list-objects-vuln"); // @source(pkg-gcs-control-list-objects-vuln)
    StorageControl.list_objects().set_parent(&v).send(); // @sink(pkg-gcs-control-list-objects-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_resource(&m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("pkg-gcs-control-list-objects-fixed"))))));
    StorageControl.list_objects().set_parent(&v).send(); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-gcs-control-list-objects-decoy"); // @source(pkg-gcs-control-list-objects-decoy)
    StorageControl.list_objects().set_parent("constant").send(); // @sink(none) @nosink(pkg-gcs-control-list-objects-decoy)
    m::sink_any(&t); // @sink(pkg-gcs-control-list-objects-decoy)
}
