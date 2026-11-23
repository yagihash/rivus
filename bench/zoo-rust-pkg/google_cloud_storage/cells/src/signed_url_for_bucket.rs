// @cell(F/pkg/google-cloud-storage/signed-url-for-bucket)
// @shape: a value of a source is the name of the bucket of SignedUrlBuilder::for_bucket (no receiver)
use zoo_mark as m;
use google_cloud_storage::builder::storage::SignedUrlBuilder;

pub fn register() {
    m::case("F/pkg/google-cloud-storage/signed-url-for-bucket/vuln", vuln);
    m::case("F/pkg/google-cloud-storage/signed-url-for-bucket/fixed", fixed);
    m::case("F/pkg/google-cloud-storage/signed-url-for-bucket/decoy", decoy);
}

fn vuln() { // @vuln
    let v = m::src("pkg-gcs-signed-url-for-bucket-vuln"); // @source(pkg-gcs-signed-url-for-bucket-vuln)
    SignedUrlBuilder::for_bucket(&v); // @sink(pkg-gcs-signed-url-for-bucket-vuln)
}

fn fixed() { // @fixed
    let v = m::clean_resource(&m::clean_path(&m::clean_html(&m::clean_sql(&m::clean_cmd(&m::src("pkg-gcs-signed-url-for-bucket-fixed"))))));
    SignedUrlBuilder::for_bucket(&v); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("pkg-gcs-signed-url-for-bucket-decoy"); // @source(pkg-gcs-signed-url-for-bucket-decoy)
    SignedUrlBuilder::for_bucket("constant"); // @sink(none) @nosink(pkg-gcs-signed-url-for-bucket-decoy)
    m::sink_any(&t); // @sink(pkg-gcs-signed-url-for-bucket-decoy)
}
