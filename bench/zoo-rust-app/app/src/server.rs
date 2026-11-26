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
            sink_cmd(&file); // from_entry @miss(not_from_entry) @ref(#234)
        }
        config.register_resource(web::to(serve));
    }
}

pub struct upload;
impl HttpServiceFactory for upload {
    fn register(self, config: &mut AppService) {
        fn upload() {
            let file = src("app-upload");
            sink_cmd(&file); // from_entry @miss(not_from_entry) @ref(#234)
        }
        config.register_resource(web::to(upload));
    }
}

pub struct delete;
impl HttpServiceFactory for delete {
    fn register(self, config: &mut AppService) {
        fn delete() {
            let file = src("app-delete");
            sink_cmd(&file); // from_entry @miss(not_from_entry) @ref(#234)
        }
        config.register_resource(web::to(delete));
    }
}

// a handler that no route registers: nothing that main reaches gives it to the config, and no framework is told of it, so its finding stays
// not_from_entry (the contrast: if every finding here becomes from_entry, the label means nothing)
pub struct orphan;
impl HttpServiceFactory for orphan {
    fn register(self, config: &mut AppService) {
        fn orphan() {
            let file = src("app-orphan");
            sink_cmd(&file); // not_from_entry
        }
        config.register_resource(web::to(orphan));
    }
}
