// @cell(F/state/receiver-field-across-entries)
// @shape: two entry points share an application state (Arc<AppState> with a Mutex field); one writes the field through the receiver, the other reads it and passes it to a sink
use std::sync::{Arc, Mutex};
use zoo_mark as m;

pub fn register() {
    m::case("F/state/receiver-field-across-entries/vuln", vuln);
    m::case("F/state/receiver-field-across-entries/fixed", fixed);
    m::case("F/state/receiver-field-across-entries/decoy", decoy);
}

// the shape of web::Data<AppState>: a shared state that the handlers get as a parameter
struct AppState {
    cache: Mutex<String>,
    other: Mutex<String>,
}

fn new_state() -> Arc<AppState> {
    Arc::new(AppState { cache: Mutex::new(String::new()), other: Mutex::new(String::new()) })
}

// entry A: the query is stored in the state
fn handler_a(state: Arc<AppState>, q: m::In) { // @source(state-receiver-field-across-entries-vuln) @miss(silent) @ref(R-58,PARITY)
    state.cache.lock().unwrap().push_str(&q.value);
}

// entry B: the state is read and goes to a sink
fn handler_b(state: Arc<AppState>) {
    let v = state.cache.lock().unwrap().clone();
    m::sink_cmd(&v); // @sink(state-receiver-field-across-entries-vuln)
}

fn vuln() { // @vuln
    let st = new_state();
    handler_a(st.clone(), m::In::new("state-receiver-field-across-entries-vuln"));
    handler_b(st);
}

fn handler_a_fixed(state: Arc<AppState>, q: m::In) {
    state.cache.lock().unwrap().push_str(&m::clean_cmd(&q.value));
}

fn handler_b_fixed(state: Arc<AppState>) {
    let v = state.cache.lock().unwrap().clone();
    m::sink_cmd(&v); // @sink(none)
}

fn fixed() { // @fixed
    let st = new_state();
    handler_a_fixed(st.clone(), m::In::new("state-receiver-field-across-entries-fixed"));
    handler_b_fixed(st);
}

// the value goes to the field `other`, and the reader that must stay quiet reads `cache`
fn handler_a_decoy(state: Arc<AppState>, q: m::In) { // @source(state-receiver-field-across-entries-decoy) @miss(silent) @ref(R-58,PARITY)
    state.other.lock().unwrap().push_str(&q.value);
}

fn handler_b_decoy(state: Arc<AppState>) {
    let c = state.cache.lock().unwrap().clone();
    let o = state.other.lock().unwrap().clone();
    m::sink_cmd(&c); // @sink(none) @nosink(state-receiver-field-across-entries-decoy)
    m::sink_sql(&o); // @sink(state-receiver-field-across-entries-decoy)
}

fn decoy() { // @decoy
    let st = new_state();
    st.cache.lock().unwrap().push_str("constant");
    handler_a_decoy(st.clone(), m::In::new("state-receiver-field-across-entries-decoy"));
    handler_b_decoy(st);
}
