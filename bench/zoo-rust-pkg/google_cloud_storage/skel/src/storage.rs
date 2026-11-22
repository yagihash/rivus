//! `client::Storage` and the builders of the data plane. The real ones take `B: Into<String>, O: Into<String>` (the bucket in
//! `projects/_/buckets/{id}` form, the object name) and return a builder whose `send()` does the request.
pub struct Storage;

pub struct ReadObject {
    pub bucket: String,
    pub object: String,
}

pub struct WriteObject {
    pub bucket: String,
    pub object: String,
}

pub struct OpenObject {
    pub bucket: String,
    pub object: String,
}

impl Storage {
    #[track_caller]
    pub fn read_object<B, O>(&self, bucket: B, object: O) -> ReadObject
    where
        B: Into<String>,
        O: Into<String>,
    {
        let (bucket, object) = (bucket.into(), object.into());
        crate::record(&bucket);
        crate::record(&object);
        ReadObject { bucket, object }
    }

    #[track_caller]
    pub fn write_object<B, O, T>(&self, bucket: B, object: O, _payload: T) -> WriteObject
    where
        B: Into<String>,
        O: Into<String>,
    {
        let (bucket, object) = (bucket.into(), object.into());
        crate::record(&bucket);
        crate::record(&object);
        WriteObject { bucket, object }
    }

    #[track_caller]
    pub fn open_object<B, O>(&self, bucket: B, object: O) -> OpenObject
    where
        B: Into<String>,
        O: Into<String>,
    {
        let (bucket, object) = (bucket.into(), object.into());
        crate::record(&bucket);
        crate::record(&object);
        OpenObject { bucket, object }
    }
}
