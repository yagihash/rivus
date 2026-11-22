// @cell(F/pkg/google-cloud-storage/signed-url-for-object)
// @shape: a value of a source is the name of the object of SignedUrlBuilder::for_object (no receiver)
use zoo_mark as m;
use google_cloud_storage::builder::storage::SignedUrlBuilder;

pub fn register() {
    m::case("F/pkg/google-cloud-storage/signed-url-for-object/vuln", vuln);
    m::case("F/pkg/google-cloud-storage/signed-url-for-object/fixed", fixed);
    m::case("F/pkg/google-cloud-storage/signed-url-for-object/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-gcs-signed-url-for-object-vuln"); // @source(pkg-gcs-signed-url-for-object-vuln)
    SignedUrlBuilder::for_object("bucket", &v); // @sink(pkg-gcs-signed-url-for-object-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("pkg-gcs-signed-url-for-object-fixed")))));
    SignedUrlBuilder::for_object("bucket", &v); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-gcs-signed-url-for-object-decoy"); // @source(pkg-gcs-signed-url-for-object-decoy)
    SignedUrlBuilder::for_object("bucket", "constant"); // @sink(none) @nosink(pkg-gcs-signed-url-for-object-decoy)
    m::sink_any(&t); // @sink(pkg-gcs-signed-url-for-object-decoy)
}
