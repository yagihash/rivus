// @cell(F/state/global-chain)
// @shape: a value goes through a chain of statics, one function per stage (3, 13 and 30 stages)
use std::sync::Mutex;
use zoo_mark as m;

pub fn register() {
    m::case("F/state/global-chain/vuln", vuln);
    m::case("F/state/global-chain/vuln13", vuln13);
    m::case("F/state/global-chain/vuln30", vuln30);
    m::case("F/state/global-chain/fixed", fixed);
    m::case("F/state/global-chain/decoy", decoy);
}

static C3G0: Mutex<String> = Mutex::new(String::new());
static C3G1: Mutex<String> = Mutex::new(String::new());
static C3G2: Mutex<String> = Mutex::new(String::new());
static C3G3: Mutex<String> = Mutex::new(String::new());
fn c3s1() {
    *C3G1.lock().unwrap() = C3G0.lock().unwrap().clone();
}
fn c3s2() {
    *C3G2.lock().unwrap() = C3G1.lock().unwrap().clone();
}
fn c3s3() {
    *C3G3.lock().unwrap() = C3G2.lock().unwrap().clone();
}

fn vuln() { // @vuln
    *C3G0.lock().unwrap() = m::src("state-global-chain-vuln"); // @source(state-global-chain-vuln)
    c3s1();
    c3s2();
    c3s3();
    m::sink_cmd(&C3G3.lock().unwrap()); // @sink(state-global-chain-vuln)
}

static C13G0: Mutex<String> = Mutex::new(String::new());
static C13G1: Mutex<String> = Mutex::new(String::new());
static C13G2: Mutex<String> = Mutex::new(String::new());
static C13G3: Mutex<String> = Mutex::new(String::new());
static C13G4: Mutex<String> = Mutex::new(String::new());
static C13G5: Mutex<String> = Mutex::new(String::new());
static C13G6: Mutex<String> = Mutex::new(String::new());
static C13G7: Mutex<String> = Mutex::new(String::new());
static C13G8: Mutex<String> = Mutex::new(String::new());
static C13G9: Mutex<String> = Mutex::new(String::new());
static C13G10: Mutex<String> = Mutex::new(String::new());
static C13G11: Mutex<String> = Mutex::new(String::new());
static C13G12: Mutex<String> = Mutex::new(String::new());
static C13G13: Mutex<String> = Mutex::new(String::new());
fn c13s1() {
    *C13G1.lock().unwrap() = C13G0.lock().unwrap().clone();
}
fn c13s2() {
    *C13G2.lock().unwrap() = C13G1.lock().unwrap().clone();
}
fn c13s3() {
    *C13G3.lock().unwrap() = C13G2.lock().unwrap().clone();
}
fn c13s4() {
    *C13G4.lock().unwrap() = C13G3.lock().unwrap().clone();
}
fn c13s5() {
    *C13G5.lock().unwrap() = C13G4.lock().unwrap().clone();
}
fn c13s6() {
    *C13G6.lock().unwrap() = C13G5.lock().unwrap().clone();
}
fn c13s7() {
    *C13G7.lock().unwrap() = C13G6.lock().unwrap().clone();
}
fn c13s8() {
    *C13G8.lock().unwrap() = C13G7.lock().unwrap().clone();
}
fn c13s9() {
    *C13G9.lock().unwrap() = C13G8.lock().unwrap().clone();
}
fn c13s10() {
    *C13G10.lock().unwrap() = C13G9.lock().unwrap().clone();
}
fn c13s11() {
    *C13G11.lock().unwrap() = C13G10.lock().unwrap().clone();
}
fn c13s12() {
    *C13G12.lock().unwrap() = C13G11.lock().unwrap().clone();
}
fn c13s13() {
    *C13G13.lock().unwrap() = C13G12.lock().unwrap().clone();
}

fn vuln13() { // @vuln
    *C13G0.lock().unwrap() = m::src("state-global-chain-vuln13"); // @source(state-global-chain-vuln13) @miss(recursion_approximation) @ref(R-44)
    c13s1();
    c13s2();
    c13s3();
    c13s4();
    c13s5();
    c13s6();
    c13s7();
    c13s8();
    c13s9();
    c13s10();
    c13s11();
    c13s12();
    c13s13();
    m::sink_cmd(&C13G13.lock().unwrap()); // @sink(state-global-chain-vuln13)
}

static C30G0: Mutex<String> = Mutex::new(String::new());
static C30G1: Mutex<String> = Mutex::new(String::new());
static C30G2: Mutex<String> = Mutex::new(String::new());
static C30G3: Mutex<String> = Mutex::new(String::new());
static C30G4: Mutex<String> = Mutex::new(String::new());
static C30G5: Mutex<String> = Mutex::new(String::new());
static C30G6: Mutex<String> = Mutex::new(String::new());
static C30G7: Mutex<String> = Mutex::new(String::new());
static C30G8: Mutex<String> = Mutex::new(String::new());
static C30G9: Mutex<String> = Mutex::new(String::new());
static C30G10: Mutex<String> = Mutex::new(String::new());
static C30G11: Mutex<String> = Mutex::new(String::new());
static C30G12: Mutex<String> = Mutex::new(String::new());
static C30G13: Mutex<String> = Mutex::new(String::new());
static C30G14: Mutex<String> = Mutex::new(String::new());
static C30G15: Mutex<String> = Mutex::new(String::new());
static C30G16: Mutex<String> = Mutex::new(String::new());
static C30G17: Mutex<String> = Mutex::new(String::new());
static C30G18: Mutex<String> = Mutex::new(String::new());
static C30G19: Mutex<String> = Mutex::new(String::new());
static C30G20: Mutex<String> = Mutex::new(String::new());
static C30G21: Mutex<String> = Mutex::new(String::new());
static C30G22: Mutex<String> = Mutex::new(String::new());
static C30G23: Mutex<String> = Mutex::new(String::new());
static C30G24: Mutex<String> = Mutex::new(String::new());
static C30G25: Mutex<String> = Mutex::new(String::new());
static C30G26: Mutex<String> = Mutex::new(String::new());
static C30G27: Mutex<String> = Mutex::new(String::new());
static C30G28: Mutex<String> = Mutex::new(String::new());
static C30G29: Mutex<String> = Mutex::new(String::new());
static C30G30: Mutex<String> = Mutex::new(String::new());
fn c30s1() {
    *C30G1.lock().unwrap() = C30G0.lock().unwrap().clone();
}
fn c30s2() {
    *C30G2.lock().unwrap() = C30G1.lock().unwrap().clone();
}
fn c30s3() {
    *C30G3.lock().unwrap() = C30G2.lock().unwrap().clone();
}
fn c30s4() {
    *C30G4.lock().unwrap() = C30G3.lock().unwrap().clone();
}
fn c30s5() {
    *C30G5.lock().unwrap() = C30G4.lock().unwrap().clone();
}
fn c30s6() {
    *C30G6.lock().unwrap() = C30G5.lock().unwrap().clone();
}
fn c30s7() {
    *C30G7.lock().unwrap() = C30G6.lock().unwrap().clone();
}
fn c30s8() {
    *C30G8.lock().unwrap() = C30G7.lock().unwrap().clone();
}
fn c30s9() {
    *C30G9.lock().unwrap() = C30G8.lock().unwrap().clone();
}
fn c30s10() {
    *C30G10.lock().unwrap() = C30G9.lock().unwrap().clone();
}
fn c30s11() {
    *C30G11.lock().unwrap() = C30G10.lock().unwrap().clone();
}
fn c30s12() {
    *C30G12.lock().unwrap() = C30G11.lock().unwrap().clone();
}
fn c30s13() {
    *C30G13.lock().unwrap() = C30G12.lock().unwrap().clone();
}
fn c30s14() {
    *C30G14.lock().unwrap() = C30G13.lock().unwrap().clone();
}
fn c30s15() {
    *C30G15.lock().unwrap() = C30G14.lock().unwrap().clone();
}
fn c30s16() {
    *C30G16.lock().unwrap() = C30G15.lock().unwrap().clone();
}
fn c30s17() {
    *C30G17.lock().unwrap() = C30G16.lock().unwrap().clone();
}
fn c30s18() {
    *C30G18.lock().unwrap() = C30G17.lock().unwrap().clone();
}
fn c30s19() {
    *C30G19.lock().unwrap() = C30G18.lock().unwrap().clone();
}
fn c30s20() {
    *C30G20.lock().unwrap() = C30G19.lock().unwrap().clone();
}
fn c30s21() {
    *C30G21.lock().unwrap() = C30G20.lock().unwrap().clone();
}
fn c30s22() {
    *C30G22.lock().unwrap() = C30G21.lock().unwrap().clone();
}
fn c30s23() {
    *C30G23.lock().unwrap() = C30G22.lock().unwrap().clone();
}
fn c30s24() {
    *C30G24.lock().unwrap() = C30G23.lock().unwrap().clone();
}
fn c30s25() {
    *C30G25.lock().unwrap() = C30G24.lock().unwrap().clone();
}
fn c30s26() {
    *C30G26.lock().unwrap() = C30G25.lock().unwrap().clone();
}
fn c30s27() {
    *C30G27.lock().unwrap() = C30G26.lock().unwrap().clone();
}
fn c30s28() {
    *C30G28.lock().unwrap() = C30G27.lock().unwrap().clone();
}
fn c30s29() {
    *C30G29.lock().unwrap() = C30G28.lock().unwrap().clone();
}
fn c30s30() {
    *C30G30.lock().unwrap() = C30G29.lock().unwrap().clone();
}

fn vuln30() { // @vuln
    *C30G0.lock().unwrap() = m::src("state-global-chain-vuln30"); // @source(state-global-chain-vuln30) @miss(recursion_approximation) @ref(R-44)
    c30s1();
    c30s2();
    c30s3();
    c30s4();
    c30s5();
    c30s6();
    c30s7();
    c30s8();
    c30s9();
    c30s10();
    c30s11();
    c30s12();
    c30s13();
    c30s14();
    c30s15();
    c30s16();
    c30s17();
    c30s18();
    c30s19();
    c30s20();
    c30s21();
    c30s22();
    c30s23();
    c30s24();
    c30s25();
    c30s26();
    c30s27();
    c30s28();
    c30s29();
    c30s30();
    m::sink_cmd(&C30G30.lock().unwrap()); // @sink(state-global-chain-vuln30)
}

static F3G0: Mutex<String> = Mutex::new(String::new());
static F3G1: Mutex<String> = Mutex::new(String::new());
static F3G2: Mutex<String> = Mutex::new(String::new());
static F3G3: Mutex<String> = Mutex::new(String::new());
fn f3s1() {
    *F3G1.lock().unwrap() = F3G0.lock().unwrap().clone();
}
fn f3s2() {
    *F3G2.lock().unwrap() = F3G1.lock().unwrap().clone();
}
fn f3s3() {
    *F3G3.lock().unwrap() = F3G2.lock().unwrap().clone();
}

fn fixed() { // @fixed
    *F3G0.lock().unwrap() = m::clean_cmd(&m::src("state-global-chain-fixed"));
    f3s1();
    f3s2();
    f3s3();
    m::sink_cmd(&F3G3.lock().unwrap()); // @sink(none)
}

static D3G0: Mutex<String> = Mutex::new(String::new());
static D3G1: Mutex<String> = Mutex::new(String::new());
static D3G2: Mutex<String> = Mutex::new(String::new());
static D3G3: Mutex<String> = Mutex::new(String::new());
fn d3s1() {
    *D3G1.lock().unwrap() = D3G0.lock().unwrap().clone();
}
fn d3s2() {
    *D3G2.lock().unwrap() = D3G1.lock().unwrap().clone();
}
fn d3s3() {
    *D3G3.lock().unwrap() = D3G2.lock().unwrap().clone();
}
static E3G0: Mutex<String> = Mutex::new(String::new());
static E3G1: Mutex<String> = Mutex::new(String::new());
static E3G2: Mutex<String> = Mutex::new(String::new());
static E3G3: Mutex<String> = Mutex::new(String::new());
fn e3s1() {
    *E3G1.lock().unwrap() = E3G0.lock().unwrap().clone();
}
fn e3s2() {
    *E3G2.lock().unwrap() = E3G1.lock().unwrap().clone();
}
fn e3s3() {
    *E3G3.lock().unwrap() = E3G2.lock().unwrap().clone();
}

fn decoy() { // @decoy
    *D3G0.lock().unwrap() = m::src("state-global-chain-decoy"); // @source(state-global-chain-decoy)
    *E3G0.lock().unwrap() = "constant".to_string();
    d3s1();
    d3s2();
    d3s3();
    e3s1();
    e3s2();
    e3s3();
    m::sink_cmd(&E3G3.lock().unwrap()); // @sink(none) @nosink(state-global-chain-decoy)
    m::sink_sql(&D3G3.lock().unwrap()); // @sink(state-global-chain-decoy)
}
