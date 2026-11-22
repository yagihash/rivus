//! Stands for the crate octocrab 0.54 in the zoo (docs/DESIGN-package-models.md, 6.8): the public signatures that the built-in
//! rows (internal/model/builtin/rust-octocrab.yaml) name, with bodies that only record what they were given (zoo-mark). The
//! requests (`send`, which is async in the crate) are not here: the cells stop at the builders. It is not a package of anyone.

pub mod models {
    pub mod issues {
        /// An issue (the fields that the cells use).
        #[derive(Default, Clone)]
        pub struct Issue {
            pub title: String,
            pub body: Option<String>,
        }
    }

    pub mod repos {
        /// A file of a repository (the fields that the cells use).
        #[derive(Default, Clone)]
        pub struct Content {
            pub name: String,
            pub path: String,
            pub content: Option<String>,
        }

        impl Content {
            /// The text of the file (the crate decodes the base64 of `content`; the skeleton returns it as it is).
            pub fn decoded_content(&self) -> Option<String> {
                self.content.clone()
            }
        }
    }
}

pub mod repos {
    use super::Octocrab;

    pub struct RepoHandler<'octo> {
        _crab: &'octo Octocrab,
    }

    impl<'octo> RepoHandler<'octo> {
        pub(crate) fn new(crab: &'octo Octocrab) -> Self {
            RepoHandler { _crab: crab }
        }

        pub fn get_content(&self) -> GetContentBuilder<'octo, '_> {
            GetContentBuilder { _handler: self }
        }

        #[track_caller]
        pub fn create_file(
            &self,
            path: impl Into<String>,
            message: impl Into<String>,
            content: impl AsRef<[u8]>,
        ) -> UpdateFileBuilder<'octo, '_> {
            let _ = (message.into(), content.as_ref());
            zoo_mark::sink_cmd(&path.into());
            UpdateFileBuilder { _handler: self }
        }
    }

    pub use file::{GetContentBuilder, UpdateFileBuilder};

    pub mod file {
        use super::RepoHandler;

        pub struct GetContentBuilder<'octo, 'r> {
            pub(crate) _handler: &'r RepoHandler<'octo>,
        }

        impl<'octo, 'r> GetContentBuilder<'octo, 'r> {
            #[track_caller]
            pub fn path(self, path: impl Into<String>) -> Self {
                zoo_mark::sink_cmd(&path.into());
                self
            }

            #[track_caller]
            pub fn r#ref(self, r#ref: impl Into<String>) -> Self {
                zoo_mark::sink_cmd(&r#ref.into());
                self
            }
        }

        pub struct UpdateFileBuilder<'octo, 'r> {
            pub(crate) _handler: &'r RepoHandler<'octo>,
        }

        impl<'octo, 'r> UpdateFileBuilder<'octo, 'r> {
            #[track_caller]
            pub fn branch(self, branch: impl Into<String>) -> Self {
                zoo_mark::sink_cmd(&branch.into());
                self
            }
        }
    }
}

#[derive(Default)]
pub struct Octocrab;

impl Octocrab {
    /// The repository that the calls after it reach: the owner and the name are put in the path of every request.
    #[track_caller]
    pub fn repos(&self, owner: impl Into<String>, repo: impl Into<String>) -> repos::RepoHandler<'_> {
        zoo_mark::sink_cmd(&owner.into());
        zoo_mark::sink_cmd(&repo.into());
        repos::RepoHandler::new(self)
    }
}
