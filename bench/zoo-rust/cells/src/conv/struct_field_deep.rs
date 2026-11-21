// @cell(F/conv/struct-field-deep)
// @shape: the request of a generated API (an optional box of a struct that holds an optional oneof, an enum whose variant is the request) is given to the call, and the rule of the sink names the URL, several steps down (docs/DESIGN-field-sinks.md)
use zoo_mark as m;

pub fn register() {
    m::case("F/conv/struct-field-deep/vuln", vuln);
    m::case("F/conv/struct-field-deep/fixed", fixed);
    m::case("F/conv/struct-field-deep/decoy", decoy);
}

fn build(h: m::HttpRequest) -> m::CreateTask {
    m::CreateTask {
        parent: String::new(),
        task: Some(Box::new(m::Task { name: String::new(), message_type: Some(m::TaskMessage::HttpRequest(h)) })),
    }
}

fn vuln() { // @vuln
    let h = m::HttpRequest { url: m::src("conv-struct-field-deep-vuln"), body: String::new() }; // @source(conv-struct-field-deep-vuln) @ref(PARITY)
    m::sink_task(&build(h)); // @sink(conv-struct-field-deep-vuln)
}

fn fixed() { // @fixed
    let h = m::HttpRequest { url: m::clean_cmd(&m::src("conv-struct-field-deep-fixed")), body: String::new() };
    m::sink_task(&build(h)); // @sink(none)
}

fn decoy() { // @decoy
    let t = m::src("conv-struct-field-deep-decoy"); // @source(conv-struct-field-deep-decoy)
    let h = m::HttpRequest { url: "https://example.com/".to_string(), body: t.clone() };
    m::sink_task(&build(h)); // @sink(none) @fp @nosink(conv-struct-field-deep-decoy) @fp @ref(FIELD-SINK,PARITY)
    m::sink_cmd(&t); // @sink(conv-struct-field-deep-decoy)
}
