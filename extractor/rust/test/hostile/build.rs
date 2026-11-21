// A build script that is hostile on purpose (see Cargo.toml). Each attempt is reported as a warning of cargo:
// `HOSTILE <name>=BLOCKED` is what the isolation is for, `HOSTILE <name>=ESCAPED` is a failure of it.
//
// The test (cmd/rivus/isolate_test.go) writes canary.txt next to this file before it runs: the line 1 is a file of
// the host, outside the project, with a secret; the line 2 is a file next to the project directory (a sibling);
// the line 3 is a value that the test put in the environment of rivus.
use std::net::{TcpStream, ToSocketAddrs};
use std::path::Path;
use std::time::Duration;

static REPORTS: std::sync::Mutex<Vec<String>> = std::sync::Mutex::new(Vec::new());

// The report goes in the environment of the compilation of the crate (HOSTILE_REPORT): the extractor builds with
// --quiet, which hides warnings, and the function in src/lib.rs that returns it puts it in the IR.
fn report(name: &str, escaped: bool) {
    let line = format!("HOSTILE {}={}", name, if escaped { "ESCAPED" } else { "BLOCKED" });
    println!("cargo:warning={}", line);
    REPORTS.lock().unwrap().push(line);
}

fn main() {
    // In the image of the isolation only. Anywhere else this does nothing, whoever runs it.
    if !Path::new("/etc/rivus-isolated").exists() {
        panic!("the build script that is hostile on purpose is for the container of rivus only");
    }
    let dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
    let canary = std::fs::read_to_string(format!("{}/canary.txt", dir)).expect("canary.txt");
    let lines: Vec<&str> = canary.lines().collect();

    // 1. the files of the host outside the project
    report("read_host_file", std::fs::read_to_string(lines[0]).is_ok());
    report("read_sibling_of_project", std::fs::read_to_string(lines[1]).is_ok());
    let parent = Path::new(&dir).parent().unwrap();
    let siblings = std::fs::read_dir(parent).map(|d| d.count()).unwrap_or(0);
    report("list_parent_of_project", siblings > 1);
    for p in ["/var/run/docker.sock", "/run/docker.sock", "/root/.ssh", "/root/.cargo/credentials.toml"] {
        report(&format!("read{}", p.replace('/', "_")), Path::new(p).exists());
    }

    // 2. the network
    let reach = |a: &str| a.to_socket_addrs().ok().and_then(|mut x| x.next()).map(|a| TcpStream::connect_timeout(&a, Duration::from_secs(3)).is_ok()).unwrap_or(false);
    report("network_1.1.1.1", reach("1.1.1.1:53"));
    report("network_8.8.8.8", reach("8.8.8.8:53"));
    report("network_dns_name", "example.com:80".to_socket_addrs().is_ok());

    // 3. writing outside the build: the project, the root, the dependencies; /tmp is the container's own and is checked by the test on the host
    report("write_project", std::fs::write(format!("{}/pwned", dir), "x").is_ok());
    report("write_root", std::fs::write("/pwned", "x").is_ok());
    report("write_etc", std::fs::write("/etc/pwned", "x").is_ok());
    report("write_cargo_home", std::fs::write("/cargo-home/pwned", "x").is_ok());
    report("write_usr_bin", std::fs::write("/usr/local/bin/pwned", "x").is_ok());
    let _ = std::fs::write(format!("/tmp/rivus-hostile-{}", lines[2]), "x");
    // what it may do: write the output of the build
    let out = std::env::var("OUT_DIR").unwrap();
    if std::fs::write(format!("{}/ok", out), "x").is_err() {
        panic!("the build script cannot write its own OUT_DIR: the container is too tight, the test says nothing");
    }

    // 4. the environment
    let secret = lines[2];
    let leaked_value = std::env::vars().any(|(_, v)| v.contains(secret));
    report("env_value_of_host", leaked_value);
    for n in ["GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY", "SSH_AUTH_SOCK", "RIVUS_HOSTILE_CANARY", "ANTHROPIC_API_KEY", "CARGO_REGISTRY_TOKEN"] {
        report(&format!("env_{}", n), std::env::var_os(n).is_some());
    }
    let home = std::env::var("HOME").unwrap_or_default();
    report("env_home_is_the_hosts", home.starts_with("/Users/") || home.starts_with("/home/") || home == "/root");

    // 4b. what can be seen: the names of the environment, the mounts, the files that can be read
    let mut names: Vec<String> = std::env::vars().map(|(k, _)| k).collect();
    names.sort();
    REPORTS.lock().unwrap().push(format!("ENV_NAMES {}", names.join(",")));
    let mounts = std::fs::read_to_string("/proc/self/mountinfo").unwrap_or_default();
    for l in mounts.lines() {
        // mount point is the fifth field
        if let Some(dst) = l.split_whitespace().nth(4) {
            let ro = l.split_whitespace().nth(5).map_or(false, |o| o.split(',').any(|x| x == "ro"));
            REPORTS.lock().unwrap().push(format!("MOUNT {} {}", dst, if ro { "ro" } else { "rw" }));
        }
    }
    let (mut files, mut suspicious) = (0usize, Vec::<String>::new());
    let mut stack = vec![std::path::PathBuf::from("/")];
    while let Some(d) = stack.pop() {
        let Ok(rd) = std::fs::read_dir(&d) else { continue };
        for e in rd.flatten() {
            let p = e.path();
            let s = p.to_string_lossy().to_string();
            if ["/proc", "/sys", "/dev"].iter().any(|x| s == *x) {
                continue;
            }
            let Ok(t) = e.file_type() else { continue };
            if t.is_dir() {
                stack.push(p);
            } else {
                if std::fs::File::open(&p).is_ok() {
                    files += 1;
                }
                let name = e.file_name().to_string_lossy().to_string();
                if [".ssh", ".gitconfig", ".git-credentials", "credentials", "credentials.toml", "docker.sock", "id_rsa", "id_ed25519", ".netrc", ".npmrc", ".aws"].contains(&name.as_str())
                    && !s.starts_with("/opt/rustup")
                {
                    suspicious.push(s);
                }
            }
        }
    }
    REPORTS.lock().unwrap().push(format!("FILES_READABLE {}", files));
    REPORTS.lock().unwrap().push(format!("SUSPICIOUS {}", suspicious.join(",")));

    // 4c. the directory of the fragments of the IR is the build's own: what is put there is read by the extractor, in the container
    if let Ok(frags) = std::env::var("RIVUS_IR_DIR") {
        let _ = std::os::unix::fs::symlink("/etc/passwd", format!("{}/zz-link-0.jsonl", frags));
        let _ = std::os::unix::fs::symlink(lines[0], format!("{}/zz-link-1.jsonl", frags));
        let _ = std::fs::write(format!("{}/unexpected name", frags), "not json\n");
        let _ = std::fs::write(format!("{}/zz-big-0.jsonl", frags), vec![b'x'; 20_000_000]);
    }

    // 4d. resources: the test names what the build script does after it has reported
    match lines.get(3).copied().unwrap_or("") {
        "forkbomb" => {
            let mut n = 0;
            let mut kids = Vec::new();
            loop {
                match std::process::Command::new("sleep").arg("300").spawn() {
                    Ok(k) => {
                        kids.push(k);
                        n += 1;
                    }
                    Err(e) => panic!("FORKBOMB stopped after {} processes: {}", n, e),
                }
                if n > 100_000 {
                    break;
                }
            }
        }
        "hang" => loop {
            std::thread::sleep(Duration::from_secs(3600));
        },
        "memory" => {
            let mut v: Vec<Vec<u8>> = Vec::new();
            loop {
                let mut b = vec![1u8; 64 << 20];
                b[0] = v.len() as u8;
                v.push(b);
            }
        }
        _ => {}
    }

    // 5. rights
    let status = std::fs::read_to_string("/proc/self/status").unwrap_or_default();
    let field = |k: &str| status.lines().find_map(|l| l.strip_prefix(k)).map(|v| v.trim().to_string()).unwrap_or_default();
    report("running_as_root", field("Uid:").split_whitespace().next() == Some("0"));
    report("has_capabilities", field("CapEff:").trim_start_matches('0') != "");
    println!("cargo:rustc-env=HOSTILE_REPORT={}", REPORTS.lock().unwrap().join(";"));
    println!("cargo:rerun-if-changed=build.rs");
}
