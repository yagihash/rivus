//! `client::StorageControl` and its request builders (`builder::storage_control`): the bucket and the object are set by methods of
//! the builder (`mut self`), which take `T: Into<String>`; `send()` does the request.
pub struct StorageControl;

#[derive(Default)]
pub struct GetObject;

#[derive(Default)]
pub struct DeleteObject;

#[derive(Default)]
pub struct MoveObject;

#[derive(Default)]
pub struct RewriteObject;

#[derive(Default)]
pub struct ListObjects;

impl StorageControl {
    pub fn get_object(&self) -> GetObject {
        GetObject
    }

    pub fn delete_object(&self) -> DeleteObject {
        DeleteObject
    }

    pub fn move_object(&self) -> MoveObject {
        MoveObject
    }

    pub fn rewrite_object(&self) -> RewriteObject {
        RewriteObject
    }

    pub fn list_objects(&self) -> ListObjects {
        ListObjects
    }

}

impl GetObject {
    #[track_caller]
    pub fn set_bucket<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    #[track_caller]
    pub fn set_object<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    pub fn send(self) {}
}

impl DeleteObject {
    #[track_caller]
    pub fn set_bucket<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    #[track_caller]
    pub fn set_object<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    pub fn send(self) {}
}

impl MoveObject {
    #[track_caller]
    pub fn set_bucket<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    #[track_caller]
    pub fn set_source_object<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    #[track_caller]
    pub fn set_destination_object<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    pub fn send(self) {}
}

impl RewriteObject {
    #[track_caller]
    pub fn set_source_bucket<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    #[track_caller]
    pub fn set_source_object<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    #[track_caller]
    pub fn set_destination_bucket<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    #[track_caller]
    pub fn set_destination_name<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    pub fn send(self) {}
}

impl ListObjects {
    #[track_caller]
    pub fn set_parent<T: Into<String>>(self, v: T) -> Self {
        crate::record(&v.into());
        self
    }

    pub fn send(self) {}
}
