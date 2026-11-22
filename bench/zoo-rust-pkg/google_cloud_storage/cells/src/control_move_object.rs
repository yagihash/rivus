// @cell(F/pkg/google-cloud-storage/control-move-object)
// @shape: a value of a source is the name of the destination object of move_object().set_destination_object of StorageControl
use zoo_mark as m;
use google_cloud_storage::client::StorageControl;

pub fn register() {
    m::case("F/pkg/google-cloud-storage/control-move-object/vuln", vuln);
    m::case("F/pkg/google-cloud-storage/control-move-object/fixed", fixed);
    m::case("F/pkg/google-cloud-storage/control-move-object/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-gcs-control-move-object-vuln"); // @source(pkg-gcs-control-move-object-vuln)
    StorageControl.move_object().set_bucket("bucket").set_source_object("a").set_destination_object(&v).send(); // @sink(pkg-gcs-control-move-object-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("pkg-gcs-control-move-object-fixed")))));
    StorageControl.move_object().set_bucket("bucket").set_source_object("a").set_destination_object(&v).send(); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-gcs-control-move-object-decoy"); // @source(pkg-gcs-control-move-object-decoy)
    StorageControl.move_object().set_bucket("bucket").set_source_object("a").set_destination_object("constant").send(); // @sink(none) @nosink(pkg-gcs-control-move-object-decoy)
    m::sink_any(&t); // @sink(pkg-gcs-control-move-object-decoy)
}
