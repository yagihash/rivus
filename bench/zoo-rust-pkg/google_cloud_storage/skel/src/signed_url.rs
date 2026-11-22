//! `builder::storage::SignedUrlBuilder`: `for_object(bucket, object)` and `for_bucket(bucket)` have no receiver.
pub struct SignedUrlBuilder;

impl SignedUrlBuilder {
    #[track_caller]
    pub fn for_object<B, O>(bucket: B, object: O) -> Self
    where
        B: Into<String>,
        O: Into<String>,
    {
        crate::record(&bucket.into());
        crate::record(&object.into());
        SignedUrlBuilder
    }

    #[track_caller]
    pub fn for_bucket<B: Into<String>>(bucket: B) -> Self {
        crate::record(&bucket.into());
        SignedUrlBuilder
    }
}
