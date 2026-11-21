use reach::Server;

fn main() {
    let s: Box<dyn Server> = Box::new(reach::modal::Web);
    s.init();
}
