use crate::web::Resource;

/// What a service is registered with: the handlers of the app implement `register`.
pub struct AppService {
    resources: Vec<Resource>,
}

impl AppService {
    pub fn new() -> Self {
        AppService { resources: Vec::new() }
    }

    pub fn register_resource(&mut self, resource: Resource) {
        self.resources.push(resource);
    }
}

/// The trait of a service (in actix-web, `actix_web::dev::HttpServiceFactory`: re-exported from `service`).
pub trait HttpServiceFactory {
    fn register(self, config: &mut AppService);
}

// the implementations for the tuples, as the real crate has them (a macro: one impl for each arity)
macro_rules! tuple_factory {
    ($(($n:tt, $T:ident)),+) => {
        impl<$($T: HttpServiceFactory),+> HttpServiceFactory for ($($T,)+) {
            fn register(self, config: &mut AppService) {
                $(self.$n.register(config);)+
            }
        }
    };
}

tuple_factory! { (0, A) }
tuple_factory! { (0, A), (1, B) }
tuple_factory! { (0, A), (1, B), (2, C) }
tuple_factory! { (0, A), (1, B), (2, C), (3, D) }
