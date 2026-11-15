use crate::{sink, source};

// async: a function that returns a future, whose body runs when it is awaited.

pub async fn produce() -> String {
    source() // @source(async)
}

pub async fn consume() {
    let s = produce().await;
    sink(&s); // @sink(async)
}

pub async fn pass_through(input: String) -> String {
    input
}

pub async fn chain() {
    let s = pass_through(source()).await; // @source(async-chain)
    sink(&s); // @sink(async-chain)
}
