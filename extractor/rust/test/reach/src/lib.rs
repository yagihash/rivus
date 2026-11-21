// What a program that starts at main reaches (sg.Program.Reachable, and the references of the graph),
// in the shapes of a web framework: a trait that the crate re-exports at its root and that main calls
// through `dyn`, a handler that is a function item given to a function that does not call it, and a
// closure that captured a value, made in one function and run by the framework.

pub mod modal {
    pub trait Server {
        fn init(&self);
    }

    pub struct Web;

    impl Server for Web {
        fn init(&self) {
            // a function item, as the handler of a route
            crate::Route::new().to(crate::handler);
            // a function item made a function pointer
            let p: fn() = crate::pointer_handler;
            crate::Route::new().to(p);
            // a closure that captured a value: the framework runs it, not this function
            let name = String::from("x");
            crate::App::new(move || {
                crate::Route::new().to(crate::inner_handler);
                let _ = &name;
            });
        }
    }
}

pub use modal::Server;

pub struct Route;

impl Route {
    pub fn new() -> Route {
        Route
    }

    // the framework keeps the handler, and runs it somewhere that is not read
    pub fn to<F: Fn() + 'static>(self, f: F) -> Route {
        std::mem::forget(f);
        self
    }
}

pub struct App;

impl App {
    pub fn new<F: Fn() + 'static>(factory: F) -> App {
        std::mem::forget(factory);
        App
    }
}

pub fn handler() {}

pub fn pointer_handler() {}

pub fn inner_handler() {}

pub fn unreferenced() {}
