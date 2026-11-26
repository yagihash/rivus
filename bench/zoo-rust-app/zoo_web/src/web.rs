use crate::service::{AppService, HttpServiceFactory};

/// The function of a route: kept, and called when a request comes (not here).
pub struct Resource {
    handler: Box<dyn Fn()>,
}

impl Resource {
    pub fn new<F: Fn() + 'static>(handler: F) -> Self {
        Resource { handler: Box::new(handler) }
    }

    pub fn call(&self) {
        (self.handler)()
    }
}

/// `web::ServiceConfig`: the services that the app registers.
pub struct ServiceConfig {
    app: AppService,
}

impl ServiceConfig {
    pub fn new() -> Self {
        ServiceConfig { app: AppService::new() }
    }

    pub fn service<F>(&mut self, factory: F) -> &mut Self
    where
        F: HttpServiceFactory + 'static,
    {
        factory.register(&mut self.app);
        self
    }
}

/// `web::to`: a resource made of a function.
pub fn to<F: Fn() + 'static>(handler: F) -> Resource {
    Resource::new(handler)
}
