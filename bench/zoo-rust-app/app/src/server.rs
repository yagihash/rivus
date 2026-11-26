// The handlers of the app. In the real target (rustypaste) an attribute macro makes, for each handler, a unit struct and an
// `impl HttpServiceFactory for <name>` whose `register` holds the handler (a function of the same name). Here the impls are
// written out. The trait is the one of `zoo_web::dev` (the re-export), as the real one is `actix_web::dev::HttpServiceFactory`.
#![allow(non_camel_case_types)]
use zoo_mark::{sink_cmd, src};
use zoo_web::dev::{AppService, HttpServiceFactory};
use zoo_web::web::{self, ServiceConfig};

/// Configures the routes: one `service` call for each handler.
pub fn configure_routes(cfg: &mut ServiceConfig) {
    cfg.service(index);
    cfg.service(version);
    cfg.service(list);
    cfg.service(serve);
    cfg.service(upload);
    cfg.service(delete);
}

pub struct index;
impl HttpServiceFactory for index {
    fn register(self, config: &mut AppService) {
        fn index() {}
        config.register_resource(web::to(index));
    }
}

pub struct version;
impl HttpServiceFactory for version {
    fn register(self, config: &mut AppService) {
        fn version() {}
        config.register_resource(web::to(version));
    }
}

pub struct list;
impl HttpServiceFactory for list {
    fn register(self, config: &mut AppService) {
        fn list() {}
        config.register_resource(web::to(list));
    }
}

// the handlers that read a request: a source and a sink in the handler, which a request reaches through the framework
pub struct serve;
impl HttpServiceFactory for serve {
    fn register(self, config: &mut AppService) {
        fn serve() {
            let file = src("app-serve");
            sink_cmd(&file); // from_entry
        }
        config.register_resource(web::to(serve));
    }
}

pub struct upload;
impl HttpServiceFactory for upload {
    fn register(self, config: &mut AppService) {
        fn upload() {
            let file = src("app-upload");
            sink_cmd(&file); // from_entry
        }
        config.register_resource(web::to(upload));
    }
}

pub struct delete;
impl HttpServiceFactory for delete {
    fn register(self, config: &mut AppService) {
        fn delete() {
            let file = src("app-delete");
            sink_cmd(&file); // from_entry
        }
        config.register_resource(web::to(delete));
    }
}

// a handler that no route registers. `ServiceConfig::service<F: HttpServiceFactory>` calls `register` on a type parameter, and the
// candidates of a call through a type parameter are all the impls of the trait (the rule of every call through a trait: a dyn call, a call of
// `Fn::call`), so an impl that nothing registers is reached too: an over-approximation, the side that misses less. Its finding is from_entry.
pub struct orphan;
impl HttpServiceFactory for orphan {
    fn register(self, config: &mut AppService) {
        fn orphan() {
            let file = src("app-orphan");
            sink_cmd(&file); // from_entry
        }
        config.register_resource(web::to(orphan));
    }
}

// the contrast of a trait of the same name: a module of the app has a trait `HttpServiceFactory` with the same method as the one of
// `zoo_web::service`, and a type that implements it. It is another trait (another definition path), and nothing registers the type,
// so its finding stays not_from_entry: if the two traits are taken for one, the registered handlers would give it a caller
pub mod other {
    use zoo_mark::{sink_cmd, src};
    use zoo_web::dev::AppService;

    pub trait HttpServiceFactory {
        fn register(self, config: &mut AppService);
    }

    pub struct lookalike;
    impl HttpServiceFactory for lookalike {
        fn register(self, _config: &mut AppService) {
            fn lookalike() {
                let file = src("app-lookalike");
                sink_cmd(&file); // not_from_entry
            }
            lookalike();
        }
    }

    // a dyn call of another trait of the module (the same method name), which main reaches: it gives the callee the impls of its own trait
    // and no handler of `zoo_web`, so it gives no impl of another trait a caller
    pub trait Hook {
        fn register(&self);
    }

    pub struct hooked;
    impl Hook for hooked {
        fn register(&self) {
            sink_cmd(&src("app-hooked")); // from_entry
        }
    }

    pub fn run_hook(hook: &dyn Hook) {
        hook.register();
    }

    // the contrast of a trait that nothing calls: no dyn call, no call through a type parameter, and nothing registers it, so no call
    // can reach its impl, and its finding stays not_from_entry (if every finding here becomes from_entry, the label means nothing)
    pub trait Unused {
        fn run(&self);
    }

    pub struct unused;
    impl Unused for unused {
        fn run(&self) {
            sink_cmd(&src("app-unused")); // not_from_entry
        }
    }
}
