// @cell(F/rust/async-await)
// @shape: the value goes through an async fn and an async block that are awaited (a minimal block_on of std::future and Waker::noop, no executor crate; a future that yields once)
use std::future::Future;
use std::pin::Pin;
use std::task::{Context, Poll, Waker};
use zoo_mark as m;

pub fn register() {
    m::case("F/rust/async-await/vuln", vuln);
    m::case("F/rust/async-await/fixed", fixed);
    m::case("F/rust/async-await/decoy", decoy);
    m::case("F/rust/async-await/vuln2", vuln2);
}

fn block_on<F: Future>(f: F) -> F::Output {
    let mut f = Box::pin(f);
    let mut cx = Context::from_waker(Waker::noop());
    loop {
        if let Poll::Ready(v) = f.as_mut().poll(&mut cx) {
            return v;
        }
    }
}

struct YieldOnce(bool);

impl Future for YieldOnce {
    type Output = ();
    fn poll(mut self: Pin<&mut Self>, _cx: &mut Context<'_>) -> Poll<()> {
        if self.0 {
            Poll::Ready(())
        } else {
            self.0 = true;
            Poll::Pending
        }
    }
}

async fn produce(s: String) -> String {
    YieldOnce(false).await;
    s
}

fn vuln() { // @vuln
    let out = block_on(async { produce(m::src("rust-async-await-vuln")).await }); // @source(rust-async-await-vuln) @miss(unmodeled_library_call) @ref(ASYNC)
    m::sink_cmd(&out); // @sink(rust-async-await-vuln)
}

fn vuln2() { // @vuln
    let v = m::src("rust-async-await-vuln2"); // @source(rust-async-await-vuln2)
    block_on(async move {
        let w = produce(v).await;
        m::sink_cmd(&w); // @sink(rust-async-await-vuln2)
    });
}

fn fixed() { // @fixed
    let out = block_on(async { produce(m::clean_cmd(&m::src("rust-async-await-fixed"))).await });
    m::sink_cmd(&out); // @sink(none)
}

fn decoy() { // @decoy
    let a = block_on(async { produce(m::src("rust-async-await-decoy")).await }); // @source(rust-async-await-decoy) @miss(unmodeled_library_call) @ref(ASYNC)
    let b = block_on(async { produce("constant".to_string()).await });
    m::sink_cmd(&b); // @sink(none) @nosink(rust-async-await-decoy)
    m::sink_sql(&a); // @sink(rust-async-await-decoy)
}
