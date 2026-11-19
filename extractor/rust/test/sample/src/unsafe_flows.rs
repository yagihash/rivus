use crate::{sink, source};

// What the compiler turns into memory operations and assembly keeps the flow
// of the data.

pub fn copied() {
    let s = source(); // @source(unsafe-copy)
    let mut dst = String::new();
    unsafe {
        std::ptr::copy_nonoverlapping(&s as *const String, &mut dst as *mut String, 1);
    }
    sink(&dst); // @sink(unsafe-copy)
}

pub fn assembled() {
    let s = source(); // @source(unsafe-asm)
    let p: *const String = &s;
    let out: *const String;
    unsafe {
        std::arch::asm!("/* {0} {1} */", out(reg) out, in(reg) p);
        sink(&*out); // @sink(unsafe-asm)
    }
}
