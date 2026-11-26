#!/usr/bin/env python3
"""The benches with heavy dependencies, run the way a user runs rivus: `rivus analyze` with no flag that
changes the analysis (docs/ROADMAP.md section 3). Every run adds ONE row to the one ledger of the heavy
measurements, bench/heavy-ledger.tsv (bench/heavy_ledger.py: the columns, the writer, the table), unless -no-ledger
(make: NO_LEDGER=1; for a binary with a measurement patch, or a run that is not used as a number). Local only (it is
heavy; not in CI): `make bench-deps`.

  bench/deps_bench.py [-timeout 600] [-rivus PATH] [-ledger FILE] [-no-ledger] [bench ...]
  bench/deps_bench.py [-limit S] [-cap GiB] [-ledger FILE] [-no-ledger] memos dart_services   (the pass line, below; dart_services is pass line D: 600 s, 8 GiB, converge; takes 10 minutes, select it with TARGETS)

The benches: go-s3 (bench/go-s3), gcp (bench/gcp), dart-gcp (bench/dart-gcp, run in a copy: `dart pub get
--offline`; the packages are read, never run). There is no Rust bench with its dependencies ready, so none is run.

The dependency benches (kind `deps`) run under the same watchdog as the pass line (the sampled memory of the whole
process tree, the footprint on macOS; the cap is 20 GiB, only to protect the machine, not a line), with `-timeout` of rivus (it reports what it
found; stats.timed_out) and, if rivus does not stop 120 s after that, a kill of the process group of this run only
(it is started in its own session; nothing outside it is signalled). `result`: `finished` (the run converged and was
not timed out), `timeout` (stats.timed_out, or the group was killed on the clock), `stopped_memory` (the watchdog),
`oom` (killed by the system or out of memory), `error`.

The pass line (`make bench-passline`; targets in bench/deps-targets.json): one line per target,
  PASS|PASS(余裕不足)|FAIL memos time=<s>/<limit> mem=<GiB>/<cap> converged=<yes|no> finish=<yes|no> detected=<yes|no|n/a> quiet_after_fix=<yes|no|n/a> [located=<n>]
with the default flags of rivus. The memory has three states (margin = cap - peak, GiB, before rounding): PASS (margin >= 0.15), PASS(余裕不足)
(0 <= margin < 0.15; a pass, the same exit status; `result` PASS-thin in the ledger) and FAIL (margin < 0); the cap does not move. PASS needs two separate conditions: FINISH (`finish`: it ended by itself within the time limit, the
memory of the child ended at or below the cap (b: the maximum over fine samples of the SAME-TIME SUM, over the whole process tree, of
the macOS `footprint` (from 2026-11-26; `mem_basis=footprint`; sampled every 0.5 s), or of the RSS where `footprint` is not there (Linux) or
could not be read in a run (`mem_basis=rss`; every 0.25 s; the whole run, never a mix); the RSS sum is kept beside it as `peak_tree_gb`, for
reference; a, the end-of-run peak of the main process, is printed beside it; the watchdog stops a run early, on the same number as the verdict,
and kills the group of this run only), it converged, with no function stopped on the clock: stats.load_dependent is not set)
and DETECT (`detected`, by place: at the PRE-FIX commit at least one finding of the category has its source and its sink at the
labelled places of the truth file, bench/memos/truth-*.json, `detect.mode` "location"; builtin models; `located=<n>` is the count of
such findings; n/a when the target has no pair).
`quiet_after_fix` (no finding of that category at the fixed commit; read only when detected, n/a otherwise) is
printed beside it and is NOT part of PASS/FAIL: a fix of the kind "validate, then return early" is not treated as
removing the flow until the guard mechanism (#169) exists. A target with `detect.mode` "through" (a truth file, one public fixed
vulnerability) counts per vulnerability: detected when at least one finding passes through a labelled position;
`through_tp=<n>/<N>` (the labelled positions passed through) is printed beside it as information. A target with `detect.mode` "score" (Gitea) counts per published vulnerability of its truth file (the form of bench/score.py: source
function, sink position, functions on the path; the field `vuln` groups the entries) and prints `detected=<n>/<N>` only: the count is not
part of PASS/FAIL (reference only), and neither the line nor the ledger row carries a count of findings, a list, a path or a place.
A target that is not decided yet is printed as INFO, not PASS/FAIL.
A Rust target (`lang` rust; shlex is an INFO row; rustypaste is a pass line row since 2026-11-26) shows that the road (fetch, the build under the sandbox, the extractor, the analysis)
goes through from one end to the other inside this tool; shlex is not a pass line (not in the default `TARGETS`; name it: `make bench-passline TARGETS=shlex`; rustypaste the same: `TARGETS=rustypaste`).
What the tool does for it (the same refusal and checks every time): (1) the fetch is `git clone` and checkout only; a `.cargo/config(.toml)` or a
`rust-toolchain(.toml)` anywhere in the tree stops the run before anything is built (what it says is printed); (2) `HOME` is an empty directory made
inside $BENCH_CACHE (the data of rivus for Rust goes under `$HOME/Library/Application Support/rivus`, so nothing is written in the shared places), and the
toolchain and the extractor that were built and set before are only read (`RIVUS_RUST_TOOLCHAIN_DIR`, `RIVUS_RUST_EXTRACTOR`, or the pinned nightly of
`extractor/rust/rust-toolchain.toml` under the real `~/.rustup` and `extractor/rust/target/debug/rivus-rust`; `rivus setup rust` is never run);
(3) before the run, `sandbox-exec` must deny a write, and after it `summary.isolation.mode` must be `sandbox`: if either is not so the run is a failure (the
line says `isolation_failed`, the exit status is 1; a failure before the run adds no row; `rivus analyze` ending by itself with an error is `analyze_failed`: no report, no row, exit status 1). The time and the memory of such a row are recorded and mean
nothing (a small crate; the sampled sum of the tree can miss a build that lasts under a second); the row's `settings` hold the count of every kind of uncertainty (with `unsupported_construct` and `unresolved_callee` always, even 0).
The time limit is kept by the watchdog (a kill of the group of this run at the limit: `stopped_time`); `rivus analyze` is not given `-timeout`
(its default is 0, no limit; a rivus that stopped itself at the limit would look like a run that did not converge), unless the `args` of the target say so.
Sources are fetched into $BENCH_CACHE (default ~/.cache/rivus-bench/), outside this tree; the output of the runs
goes to $BENCH_OUT (default .claude/work/bench-out/, git-ignored; the ledger holds numbers only, the JSON stays there).
Nothing of a target is run.
"""
import argparse
import contextlib
import datetime
import fcntl
import json
import os
import re
import shutil
import signal
import statistics
import subprocess
import sys
import tempfile
import time
import tomllib

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from heavylock import heavy_lock, snapshot, format_snapshots  # one lock for the heavy runs of the whole machine (bench/heavylock.py)
import rustenv  # the data of rivus for Rust is not the shared place (bench/rustenv.py)
import heavy_ledger  # the one ledger of the heavy measurements (bench/heavy_ledger.py)

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BENCHES = {
    "go-s3": ("bench/go-s3", "go"),
    "gcp": ("bench/gcp", "go"),
    "dart-gcp": ("bench/dart-gcp", "dart"),
}
DEPS_CAP_GB = 20  # the watchdog of the dependency benches: protects the machine; it is not a line
GRACE = 120


@contextlib.contextmanager
def fixed_work(cache, name):
    """The work dir of one target: `<cache>/<name>`, the same path in every run. The copy of the target is made in it, and the path is part of the
    function URIs, so the ids of the findings are the same from run to run (a random temporary dir changed them; the comparison of two runs had to
    rename it). It is emptied before and after the run; a lock file beside it refuses a second run of the same name at the same time."""
    if not re.fullmatch(r"[A-Za-z0-9._-]+", name) or name in (".", ".."):  # the name may come from the command line: it is removed below
        raise SystemExit("deps_bench: refused a work dir name %r (letters, digits, . _ - only)" % name)
    path = os.path.join(cache, name)
    if os.path.dirname(os.path.realpath(path)) != os.path.realpath(cache):
        raise SystemExit("deps_bench: the work dir %s is not directly under %s" % (path, cache))
    with open(path + ".lock", "w") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            raise SystemExit("deps_bench: %s is in use by another run (lock %s.lock)" % (path, path))
        shutil.rmtree(path, ignore_errors=True)
        os.makedirs(path)
        try:
            yield path
        finally:
            shutil.rmtree(path, ignore_errors=True)


def measure(name, rivus, timeout, work):
    """One dependency bench. Returns (row of the ledger without date and commit, None), or (None, why it was skipped)."""
    d, lang = BENCHES[name]
    src = os.path.join(ROOT, d)
    env = dict(os.environ)
    if lang == "dart":
        copy = os.path.join(work, name)
        shutil.copytree(src, copy)
        r = subprocess.run(["dart", "pub", "get", "--offline"], cwd=copy, capture_output=True, text=True)
        if r.returncode != 0:
            return None, "dart pub get --offline failed: " + r.stderr.strip()[-200:]
        src = copy
        env["RIVUS_DART"] = "dart"
        env["RIVUS_DART_EXTRACTOR"] = dart_extractor_command()
    cmd = [rivus, "analyze", "-lang", lang, "-C", src, "-timeout", "%ds" % timeout]
    outp, errp = os.path.join(work, name + ".out.json"), os.path.join(work, name + ".err")
    with heavy_lock("deps_bench " + name):
        load = os.getloadavg()[0]
        sn0 = snapshot()
        m = run_watched(cmd, ROOT, env, timeout + GRACE, int(DEPS_CAP_GB * GB), errp, outp)
        note = format_snapshots(sn0, snapshot())
    err = open(errp, errors="replace").read()
    rep = None
    if not m["why"]:
        try:
            rep = json.load(open(outp))
        except (OSError, ValueError):
            rep = None
    if rep is not None:  # the extractor and rivus must be of the same source (ADR-126): a failure, no row
        bad = extractor_failure(rep, lang)
        if bad:
            raise SystemExit("deps_bench: %s extractor_mismatch(%s): %s" % (name, bad[0], bad[1]))
    st = rep.get("stats", {}) if isinstance(rep, dict) else {}
    converged = bool(st.get("converged", True)) if rep is not None else False
    row = {"target": name, "target_commit": "in-tree", "kind": "deps", "settings": "default (-timeout %ds); %s" % (timeout, note),
           "wall_s": "%.1f" % m["wall"], "cpu_s": "%.1f" % m["cpu"], "peak_tree_gb": "%.2f" % (m["b_rss"] / GB),
           "peak_footprint_gb": "%.2f" % (m["b_fp"] / GB) if m["basis"] == "footprint" else None, "mem_basis": m["basis"],
           "converged": converged if rep is not None else None, "exit_code": m["rc"], "load_start": "%.2f" % load}
    row.update({k: v for k, v in heavy_ledger.report_columns(rep).items() if v is not None})
    if m["why"] == "memory":
        row["result"] = "stopped_memory"
    elif m["why"] == "time" or st.get("timed_out"):
        row["result"] = "timeout"
    elif rep is not None:
        row["result"] = "finished" if converged else "error"
    elif m["rc"] in (-9, 137) or re.search(r"out of memory|cannot allocate memory", err, re.I):
        row["result"] = "oom"
    else:
        row["result"] = "error"
    return row, None


TARGETS_FILE = os.path.join(ROOT, "bench/deps-targets.json")
GB = 1024 ** 3
EXIT_INCOMPLETE = 4  # rivus: the output is complete as far as it goes, the analysis is not (cmd/rivus exitIncomplete)


def judge(time_s, limit_s, mem_gb, cap_gb, converged, detected, through=None, quiet_after_fix=None, located=None, score=None, coarsened=None, expect_coarsened=0, detect_decides=True):
    """The pass line, pure: two separate conditions, both needed. FINISH (time within the limit, memory within the cap,
    converged) and DETECT (`detected`). located: None or the count of findings at the labelled places (information). detected: True / False / None (no pair for this target). through: None or (tp, n), the
    position-level count, printed as information only (it does not decide PASS). quiet_after_fix: True / False /
    None (n/a); printed, and NOT part of PASS/FAIL. score: None or (n, N), the vulnerabilities detected by place (detect.mode
    score), printed as `detected=<n>/<N>` only, never part of PASS/FAIL. coarsened: None or stats.scc_coarsened (the components
    whose dependency functions are solved at depth 0: -scc-coarsen, ADR-124); it must be expect_coarsened (0 for a target that is not meant
    to be hit), or the line fails: a target that comes to be hit changes its answer without being said. detect_decides: False (`detect_in_verdict`
    false in the data: rustypaste, 2026-11-26) prints `detected` but does not let it decide. Returns (passed, line_body)."""
    ok_time = time_s is not None and time_s <= limit_s
    ok_mem = mem_gb is not None and mem_gb <= cap_gb
    if score is not None:
        detected = None  # the count is printed (below) but is not part of the verdict
    ok_det = detected is not False or not detect_decides  # None (n/a) does not count against
    finish = bool(ok_time and ok_mem and converged)
    ok_coarse = coarsened is None or coarsened == expect_coarsened
    passed = bool(finish and ok_det and ok_coarse)
    yn = lambda v: "n/a" if v is None else ("yes" if v else "no")
    body = "time=%s/%s mem=%s/%s converged=%s finish=%s detected=%s quiet_after_fix=%s" % (
        "%.0f" % time_s if time_s is not None else "-", "%g" % limit_s,
        "%.2f" % mem_gb if mem_gb is not None else "-", "%g" % cap_gb,
        "yes" if converged else "no", yn(finish), yn(detected) if score is None else "%s/%d" % ("-" if score[0] is None else score[0], score[1]), yn(quiet_after_fix))
    if coarsened is not None:
        body += " coarsened=%d/%d" % (coarsened, expect_coarsened)
    if located is not None:
        body += " located=%d" % located
    if through is not None:
        body += " through_tp=%s/%d" % ("-" if through[0] is None else through[0], through[1])
    return passed, body


MARGIN_THIN_GIB = 0.15  # the margin below which a memory pass is "thin": about 1.5 times the spread of ten-odd measurements (0.05 to 0.1 GiB)


def margin_state(mem_gb, cap_gb):
    """The memory of the pass line in three states (the one place that decides it). margin = cap - peak, both in GiB, before any
    rounding. "pass": margin >= 0.15; "thin": 0 <= margin < 0.15 (still a pass); "fail": margin < 0, or no number. The line itself
    (the cap) does not move. The rounding of the difference only takes the float noise away (8 - 7.85 is 0.14999999999999947)."""
    if mem_gb is None:
        return "fail"
    margin = round(cap_gb - mem_gb, 9)
    return "fail" if margin < 0 else ("thin" if margin < MARGIN_THIN_GIB else "pass")


def result_labels(passed, decided, mem_gb, cap_gb):
    """(the value of `result` in the ledger, the word of the printed line). The ledger value is short and ASCII (PASS, PASS-thin,
    FAIL, INFO); the line says PASS(余裕不足) for a pass with a thin memory margin. A thin pass exits with the same status as PASS."""
    if not decided:
        return "INFO", "INFO"
    if not passed:
        return "FAIL", "FAIL"
    if margin_state(mem_gb, cap_gb) == "thin":
        return "PASS-thin", "PASS(余裕不足)"
    return "PASS", "PASS"


def row_failed(row):
    """A row that is no usable result (the exit status of the run is 1). FAIL and PASS-thin are results."""
    return row is None or row.get("result") == "isolation_failed"


def format_line(name, decided, passed, body, label=None):
    return "%s %s %s" % (label or (("PASS" if passed else "FAIL") if decided else "INFO"), name, body)


def count_category(rep, category):
    return sum(1 for f in rep.get("findings", []) if f.get("category") == category)


def detection(pre_count):
    """Detected: the pre-fix commit has at least one finding of the category (None: the run did not converge)."""
    return pre_count is not None and pre_count > 0


def quiet_after_fix(fixed_count):
    """The fixed commit has no finding of the category (None: the run did not converge, so not quiet)."""
    return fixed_count is not None and fixed_count == 0


def location_counts(truth, rep, category, side="prefix"):
    """Detection by place (detect.mode location; the truth is the list `sites` of bench/memos/truth-*.json: role source /
    sink, file, line = the line at the pre-fix commit, fixed_line = the line at the fixed commit). A finding counts when it is of
    the category AND its source is at a source site AND its sink is at a sink site (file, line of the position of the
    finding). Returns (findings at the places, source sites, sink sites). Counts only; the finding itself is not kept."""
    key = "line" if side == "prefix" else "fixed_line"
    at = lambda role: {(x["file"], x[key]) for x in truth if x.get("role") == role and x.get("category") == category}
    src, snk = at("source"), at("sink")
    pos = lambda e: ((e or {}).get("position") or {})
    n = 0
    for f in (rep or {}).get("findings", []):
        if f.get("category") != category:
            continue
        a, b = pos(f.get("source")), pos(f.get("sink"))
        if (a.get("file"), a.get("line")) in src and (b.get("file"), b.get("line")) in snk:
            n += 1
    return n, len(src), len(snk)


def detected_located(n):
    """Per vulnerability: detected when at least one finding is at the labelled places (None: the run did not converge)."""
    return n is not None and n >= 1


def through_counts(truth, rep):
    """Position level (the same scoring as `bench/score.py --through`): a labelled "vuln" site (file, line) counts when a
    finding of its category has a path with a `call ...` step or the final `sink: ...` step at that position (the labelled
    call is a helper on the path, or the sink itself; kinds that differ do not count). Returns (tp, n). Counts only."""
    call_steps = {}
    for f in (rep or {}).get("findings", []):
        for pa in f.get("paths") or []:
            for st in pa.get("steps", []):
                if st.get("desc", "").startswith(("call ", "sink: ")):
                    q = st["position"]
                    call_steps.setdefault((q["file"], q["line"]), set()).add(f["category"])
    sites = [t for t in truth if "file" in t and t.get("label") == "vuln"]
    tp = sum(1 for t in sites if t["category"] in call_steps.get((t["file"], t["line"]), ()))
    return tp, len(sites)


def detected_through(tp):
    """Per vulnerability: detected when at least one finding passes through a labelled position of its truth."""
    return tp is not None and tp >= 1


def score_counts(truth, rep):
    """Detection by place for detect.mode score (the form of bench/score.py: source function, sink file and line, `via`: the
    functions on one path). The truth is the list `sites` of bench/gitea-cve-truth.json; the field `vuln` groups the entries of
    one published vulnerability, which counts as detected when one of its "vuln" entries is hit. Returns (detected, N).
    Counts only: nothing of a finding is kept or returned."""
    pair = {}  # (source func, sink file, sink line, category) -> [set of the functions of each path, with the callees]
    for f in (rep or {}).get("findings", []):
        p = (f.get("sink") or {}).get("position") or {}
        fs_all = pair.setdefault(((f.get("source") or {}).get("func", ""), p.get("file"), p.get("line"), f.get("category")), [])
        for pa in f.get("paths") or []:
            fs = set()
            for st in pa.get("steps", []):
                fs.add(st.get("func", ""))
                dsc = st.get("desc", "")
                if dsc.startswith("call "):
                    fs.add(dsc[5:])
            fs_all.append(fs)

    def hit(s):
        for (fn, sf, sl, cat), paths in pair.items():
            if not (fn.endswith(s["source_func"]) and (sf, sl) == (s["sink_file"], s["sink_line"]) and cat == s["category"]):
                continue
            via = s.get("via") or []
            if not via or any(all(any(x.endswith(v) for x in fs) for v in via) for fs in paths):
                return True
        return False

    groups = {}
    for s in truth:
        if s.get("label") == "vuln":
            groups[s["vuln"]] = groups.get(s["vuln"], False) or hit(s)
    return sum(groups.values()), len(groups)


def tree_rows(root_pid, pgid):
    """The processes of the tree of the run: the process, its descendants (by parent), and the members of its group.
    Returns [(pid, rss_bytes, comm)]; ps only (nothing is signalled)."""
    r = subprocess.run(["ps", "-A", "-o", "pid=,ppid=,pgid=,rss=,comm="], capture_output=True, text=True)
    rows = {}
    kids = {}
    for l in r.stdout.splitlines():
        f = l.split(None, 4)
        if len(f) == 5 and f[0].isdigit() and f[1].isdigit() and f[2].isdigit() and f[3].isdigit():
            pid, ppid, pg = int(f[0]), int(f[1]), int(f[2])
            rows[pid] = (pg, int(f[3]) * 1024, os.path.basename(f[4]))
            kids.setdefault(ppid, []).append(pid)
    seen, todo = set(), [root_pid]
    while todo:
        x = todo.pop()
        if x in seen or x not in rows:
            continue
        seen.add(x)
        todo.extend(kids.get(x, []))
    seen |= {pid for pid, v in rows.items() if v[0] == pgid}
    return [(pid, rows[pid][1], rows[pid][2]) for pid in sorted(seen)]


def maxrss_bytes(ru):
    """ru_maxrss of a wait4 result: bytes on macOS, KB on Linux."""
    return int(ru.ru_maxrss) * (1 if sys.platform == "darwin" else 1024)


FOOTPRINT_FAILS = 3  # this many failed calls of `footprint` in a row (not a vanished pid): the run goes back to RSS as a whole


def mem_basis():
    """What the memory of the pass line is measured in: "footprint" (macOS `footprint`, the physical footprint of the process, as of
    2026-11-26: the RSS of macOS counts the pages that Go gave back with MADV_FREE_REUSABLE and the RSS peak of one target moves
    by 0.5 GiB between equal runs) or "rss" (`ps`; Linux, or no `footprint`)."""
    return "footprint" if sys.platform == "darwin" and shutil.which("footprint") else "rss"


def parse_footprint_json(text):
    """(total, name of the biggest process, its footprint), all bytes, from the JSON of `footprint -j`; None for anything broken.
    total is `total footprint` (the sum over the processes asked for, shared pages counted once)."""
    try:
        j = json.loads(text)
    except (TypeError, ValueError):
        return None
    if not isinstance(j, dict):
        return None
    tot = j.get("total footprint")
    if not isinstance(tot, int) or isinstance(tot, bool) or tot < 0:
        return None
    big, big_fp = "", 0
    for p in j.get("processes") or []:
        fp = p.get("footprint") if isinstance(p, dict) else None
        if isinstance(fp, int) and not isinstance(fp, bool) and fp > big_fp:
            big, big_fp = str(p.get("name", "")), fp
    return tot, big, big_fp


def footprint_tree(pids, tmpfile):
    """One call of `footprint -j FILE -p PID ...` for the processes of the tree. Returns ("ok", parse_footprint_json) , ("race", None)
    when a pid had gone (rc 66: this sample is dropped), or ("fail", None). Nothing is signalled; the target is not paused."""
    try:
        r = subprocess.run(["footprint", "-j", tmpfile] + [a for p in pids for a in ("-p", str(p))], capture_output=True, timeout=20)
    except (OSError, subprocess.SubprocessError):
        return "fail", None
    if r.returncode == 66:
        return "race", None
    if r.returncode != 0:
        return "fail", None
    try:
        got = parse_footprint_json(open(tmpfile, errors="replace").read())
    except OSError:
        got = None
    return ("ok", got) if got else ("fail", None)


def run_watched(cmd, cwd, env, limit_s, cap_bytes, errpath, outpath, interval=None):
    """Run cmd in its own session. Records of memory, and a WATCHDOG that is not a record:
      a = the end-of-run peak of the main process (ru_maxrss of wait4: the largest single process of its subtree; RSS-like);
      b_rss = the maximum over fine samples of the SAME-TIME SUM of the RSS of the whole process tree (`ps`; shared pages can be counted twice);
      b_fp = the same for the footprint (macOS `footprint` of the processes that `ps` found, one call per sample; 0 when not used);
      b = the number of the verdict: b_fp when basis is "footprint", else b_rss. basis is "footprint" on macOS when `footprint` works; if it
          fails FOOTPRINT_FAILS times in a row the whole run is RSS (basis "rss"), never a mix. A vanished pid (rc 66) drops that sample only.
      `big` names the biggest process of the tree at that moment (in the basis), big_rss its bytes, and `at_s` when.
    interval: 0.5 s when the basis is the footprint, 0.25 s for the RSS (ps only is cheap).
    The watchdog kills the group (only) when the wall time is over limit_s or b is over cap_bytes (the footprint when that is the basis:
    the kill and the verdict use one number).
    Returns a dict: rc, why ("" / "time" / "memory"), wall, cpu, a, b, b_rss, b_fp, basis, big, big_rss, at_s."""
    with open(outpath, "wb") as out, open(errpath, "wb") as err:
        p = subprocess.Popen(cmd, cwd=cwd, env=env, stdout=out, stderr=err, start_new_session=True)
    t0 = time.time()
    pgid = p.pid
    basis = mem_basis()
    if interval is None:
        interval = 0.5 if basis == "footprint" else 0.25
    fp_tmp = None
    if basis == "footprint":
        fd, fp_tmp = tempfile.mkstemp(prefix="footprint-", suffix=".json")
        os.close(fd)
    b_rss, b_fp, why = 0, 0, ""
    big_r, big_f = ("", 0, 0.0), ("", 0, 0.0)  # (name, bytes, at_s) of the biggest process at the time of each maximum
    fails = 0
    rc, ru = None, None
    try:
        while True:
            wp, status, ru_ = os.wait4(p.pid, os.WNOHANG)
            if wp:
                rc, ru = os.waitstatus_to_exitcode(status), ru_
                break
            rows = tree_rows(p.pid, pgid)
            tot = sum(r[1] for r in rows)
            if tot > b_rss:
                top = max(rows, key=lambda r: r[1])
                b_rss, big_r = tot, (top[2], top[1], time.time() - t0)
            if basis == "footprint" and rows:
                state, got = footprint_tree([r[0] for r in rows], fp_tmp)
                if state == "ok":
                    fails = 0
                    if got[0] > b_fp:
                        b_fp, big_f = got[0], (got[1], got[2], time.time() - t0)
                elif state == "fail":
                    fails += 1
                    if fails >= FOOTPRINT_FAILS:
                        basis, b_fp, big_f = "rss", 0, ("", 0, 0.0)  # the whole run in RSS: nothing of the footprint is kept
            b = b_fp if basis == "footprint" else b_rss
            if b > cap_bytes:
                why = "memory"
            elif time.time() - t0 > limit_s:
                why = "time"
            if why:
                if os.getpgid(p.pid) != p.pid or p.pid == os.getpgrp():
                    raise SystemExit("deps_bench: refusing to kill process group %d (not the run's own)" % pgid)
                os.killpg(pgid, signal.SIGKILL)
                _, status, ru = os.wait4(p.pid, 0)
                rc = os.waitstatus_to_exitcode(status)
                break
            time.sleep(interval)
    finally:
        if fp_tmp:
            with contextlib.suppress(OSError):
                os.unlink(fp_tmp)
    if basis == "footprint" and b_fp == 0:
        basis = "rss"  # no sample of the footprint was read (a run that ended before the first one): RSS, said so
    big = big_f if basis == "footprint" else big_r
    return {"rc": rc, "why": why, "wall": time.time() - t0, "cpu": ru.ru_utime + ru.ru_stime,
            "a": maxrss_bytes(ru), "b": b_fp if basis == "footprint" else b_rss, "b_rss": b_rss, "b_fp": b_fp if basis == "footprint" else 0,
            "basis": basis, "big": big[0], "big_rss": big[1], "at_s": big[2]}


def git(*args, cwd=None):
    # no hooks, no filters that run programs; fetch and checkout only
    subprocess.run(["git", "-c", "core.hooksPath=/dev/null"] + list(args), cwd=cwd, check=True, capture_output=True)


def cache_dir():
    d = os.environ.get("BENCH_CACHE") or os.path.expanduser("~/.cache/rivus-bench")
    d = os.path.abspath(d)
    if d == ROOT or d.startswith(ROOT + os.sep):
        raise SystemExit("deps_bench: BENCH_CACHE must be outside the repository (%s)" % d)
    os.makedirs(d, exist_ok=True)
    return d


def out_dir():
    d = os.path.abspath(os.environ.get("BENCH_OUT") or os.path.join(ROOT, ".claude/work/bench-out"))
    if d == ROOT or d.startswith(ROOT + os.sep):
        r = subprocess.run(["git", "check-ignore", "-q", os.path.join(d, "x")], cwd=ROOT)
        if r.returncode != 0:
            raise SystemExit("deps_bench: BENCH_OUT %s is inside the repository and not git-ignored" % d)
    os.makedirs(d, exist_ok=True)
    return d


def fetch(name, t, commit, cache):
    """The tree of the target at commit, in the cache. Returns its path. Fetch and checkout only."""
    base = os.path.join(cache, name)
    os.makedirs(base, exist_ok=True)
    main = os.path.join(base, "src")
    if not os.path.isdir(os.path.join(main, ".git")):
        git("clone", "--no-checkout", t["repo"], main)
    short = commit[:12]
    wt = os.path.join(base, "wt-" + short)
    if not os.path.isdir(wt):
        try:
            git("cat-file", "-e", commit + "^{commit}", cwd=main)
        except subprocess.CalledProcessError:
            git("fetch", "origin", commit, cwd=main)
        git("worktree", "add", "--detach", wt, commit, cwd=main)
    return wt


def prepare(name, t, tree, work, env):
    """Returns (analysis dir, env, error)."""
    src = os.path.join(tree, t.get("subdir", ""))
    if t["lang"] == "go":
        r = subprocess.run(["go", "mod", "download"], cwd=src, capture_output=True, text=True)
        if r.returncode != 0:
            return None, env, "go mod download failed: " + r.stderr.strip()[-200:]
        return src, env, None
    if t["lang"] == "rust":
        refuse_rust(name, tree)  # before the copy, before anything is built
        copy = os.path.join(work, os.path.basename(tree))
        if not os.path.isdir(copy):
            shutil.copytree(tree, copy, ignore=shutil.ignore_patterns(".git"))
        src = os.path.join(copy, t.get("subdir", ""))
        lock = os.path.join(src, "Cargo.lock")
        if t.get("rust_minimal_lock") and not os.path.exists(lock):  # a crate with no dependency and no Cargo.lock: the smallest lock, written in the COPY only
            body = open(os.path.join(src, "Cargo.toml")).read()
            if not rust_no_dependencies(body):
                return None, env, "rust_minimal_lock is only for a crate with no dependency, and %s names some" % name
            open(lock, "w").write(minimal_lock(body))
        elif not os.path.exists(lock):  # a crate with dependencies: the Cargo.lock that is committed, as it is; this tool never makes one (no `cargo generate-lockfile`)
            return None, env, "%s has no committed Cargo.lock (this tool does not generate one)" % name
        env = dict(env)
        env["RIVUS_RUST_TOOLCHAIN_DIR"], env["RIVUS_RUST_EXTRACTOR"] = rust_tools()
        return src, env, None
    copy = os.path.join(work, os.path.basename(tree))
    if not os.path.isdir(copy):
        shutil.copytree(tree, copy, ignore=shutil.ignore_patterns(".git"))
    if t.get("dart_remove_flutter_workspace_members"):
        remove_flutter_members(copy)
    pin_lock(t, copy)
    src = os.path.join(copy, t.get("subdir", ""))
    r = subprocess.run(["dart", "pub", "get"], cwd=src, capture_output=True, text=True)
    if r.returncode != 0:
        return None, env, "dart pub get failed: " + r.stderr.strip()[-200:]
    env = dict(env)
    env["RIVUS_DART"] = "dart"
    env["RIVUS_DART_EXTRACTOR"] = dart_extractor_command()
    return src, env, None


def pin_lock(t, root):
    """In the COPY: put the pinned `pubspec.lock` of the target (the `lock` field, a path in this repository) at the root of the tree, so `dart pub get` keeps those versions. No `lock` field: nothing is written."""
    lock = t.get("lock")
    if not lock:
        return
    shutil.copyfile(os.path.join(ROOT, lock), os.path.join(root, "pubspec.lock"))


def remove_flutter_members(root):
    """In the COPY: drop from the root pubspec the `workspace:` members that need Flutter."""
    path = os.path.join(root, "pubspec.yaml")
    lines = open(path).read().split("\n")
    out, in_ws = [], False
    for l in lines:
        if re.match(r"^workspace:\s*$", l):
            in_ws = True
            out.append(l)
            continue
        if in_ws:
            m = re.match(r"^\s*-\s*(\S+)\s*$", l)
            if m:
                pub = os.path.join(root, m.group(1), "pubspec.yaml")
                if os.path.exists(pub) and re.search(r"^\s*sdk:\s*flutter\s*$|^\s*flutter:\s*$", open(pub).read(), re.M):
                    continue
            elif l.strip():
                in_ws = False
        out.append(l)
    open(path, "w").write("\n".join(out))


RUST_CONFIG_FILES = ("rust-toolchain", "rust-toolchain.toml")
RUST_DEP_TABLES = ("dependencies", "dev-dependencies", "build-dependencies")


def rust_refusals(tree):
    """The files of a fetched Rust tree that stop the run: a `.cargo/config(.toml)` (a rustc-wrapper, a linker, a runner are programs that cargo
    runs) or a `rust-toolchain(.toml)` (another toolchain) at any depth. Returns [(path relative to tree, first 400 characters of it)]. Read only."""
    found = []
    for dp, dns, fns in os.walk(tree):
        dns[:] = [x for x in dns if x != ".git"]
        names = [f for f in fns if f in RUST_CONFIG_FILES]
        if os.path.basename(dp) == ".cargo":
            names += [f for f in fns if f in ("config", "config.toml")]
        for f in names:
            path = os.path.join(dp, f)
            try:
                body = open(path, errors="replace").read(400)
            except OSError as e:
                body = "(not readable: %s)" % e
            found.append((os.path.relpath(path, tree), body))
    return sorted(found)


def refuse_rust(name, tree):
    """Raises SystemExit, saying what the files hold, when rust_refusals finds any. Nothing was built or run."""
    found = rust_refusals(tree)
    if found:
        raise SystemExit("deps_bench: %s: refused, nothing was built or run (the build would read the configuration of the target):\n%s" % (
            name, "\n".join("  %s: %s" % (p, b.replace("\n", " | ")) for p, b in found)))


def rust_no_dependencies(cargo_toml):
    """True when the Cargo.toml (text) names no dependency of any kind (also under [target.*]) and no workspace."""
    t = tomllib.loads(cargo_toml)
    if "workspace" in t:
        return False
    if any(t.get(k) for k in RUST_DEP_TABLES):
        return False
    return not any(any(v.get(k) for k in RUST_DEP_TABLES) for v in (t.get("target") or {}).values())


def minimal_lock(cargo_toml):
    """The smallest Cargo.lock (version 4, one [[package]]) for a crate with no dependency. Only for such a crate: the caller checks."""
    pkg = tomllib.loads(cargo_toml)["package"]
    return '# This file is automatically @generated by Cargo.\n# It is not intended for manual editing.\nversion = 4\n\n[[package]]\nname = "%s"\nversion = "%s"\n' % (pkg["name"], pkg["version"])


def sandbox_denies_write(dirpath, run=subprocess.run):
    """Before a Rust run: `sandbox-exec` with a profile that denies every write must stop a `touch` (it ran and was refused: its own message, no file made).
    Returns (ok, why). `run` is a parameter only so that the tests need no sandbox."""
    probe = os.path.join(dirpath, "write-probe-%d" % os.getpid())
    try:
        r = run(["/usr/bin/sandbox-exec", "-p", "(version 1)(allow default)(deny file-write*)", "/usr/bin/touch", probe], capture_output=True, text=True)
    except OSError as e:
        return False, "sandbox-exec could not be started: %s" % e
    made = os.path.exists(probe)
    if made:
        os.remove(probe)
        return False, "a write under sandbox-exec was NOT denied (the file was made)"
    if r.returncode == 0:
        return False, "sandbox-exec ended 0 with a write that should be denied"
    if "touch:" not in (r.stderr or "") or "not permitted" not in (r.stderr or ""):
        return False, "sandbox-exec did not run the probe (%s)" % (r.stderr or "").strip()[-200:]
    return True, ""


def isolation_ok(rep):
    """After a Rust run: summary.isolation.mode is "sandbox". Returns (ok, why)."""
    mode = (((rep or {}).get("summary") or {}).get("isolation") or {}).get("mode")
    return (True, "") if mode == "sandbox" else (False, "summary.isolation.mode is %r, not 'sandbox'" % (mode,))


def uncertainty_counts(rep):
    """{kind: count} of the uncertainties of a report; unsupported_construct and unresolved_callee are always there (0 when none)."""
    n = {"unsupported_construct": 0, "unresolved_callee": 0}
    for u in (rep or {}).get("uncertainties") or []:
        n[u.get("kind", "?")] = n.get(u.get("kind", "?"), 0) + 1
    return n


def format_uncertainties(n):
    return "uncertainties " + " ".join("%s=%d" % (k, v) for k, v in sorted(n.items()))


def rust_tools():
    """(toolchain dir, extractor) to be only read, found with the REAL home (before HOME is made empty): RIVUS_RUST_TOOLCHAIN_DIR /
    RIVUS_RUST_EXTRACTOR, else the pinned nightly under ~/.rustup and extractor/rust/target/debug/rivus-rust. Raises SystemExit when one is not there."""
    tc = os.environ.get("RIVUS_RUST_TOOLCHAIN_DIR")
    if not tc:
        chan = tomllib.load(open(os.path.join(ROOT, "extractor/rust/rust-toolchain.toml"), "rb"))["toolchain"]["channel"]
        arch = "x86_64" if os.uname().machine == "x86_64" else "aarch64"
        tc = os.path.join(os.path.expanduser("~"), ".rustup", "toolchains", "%s-%s-apple-darwin" % (chan, arch))
    ex = os.environ.get("RIVUS_RUST_EXTRACTOR") or os.path.join(ROOT, "extractor/rust/target/debug/rivus-rust")
    if not os.path.isfile(os.path.join(tc, "bin", "rustc")):
        raise SystemExit("deps_bench: the pinned toolchain is not at %s (prepare it yourself, or set RIVUS_RUST_TOOLCHAIN_DIR; this tool never runs `rivus setup rust`)" % tc)
    if not os.access(ex, os.X_OK):
        raise SystemExit("deps_bench: the Rust extractor is not at %s (build extractor/rust, or set RIVUS_RUST_EXTRACTOR)" % ex)
    return tc, ex


def extractor_hash(lang):
    """The hash of the source of the extractor of lang in this tree (`go run ./scripts/extractor-hash`: the value that `make build` gives to rivus)."""
    r = subprocess.run(["go", "run", "./scripts/extractor-hash", lang], cwd=ROOT, capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit("deps_bench: go run ./scripts/extractor-hash %s failed: %s" % (lang, r.stderr.strip()[-200:]))
    return r.stdout.strip()


def dart_extractor_command():
    """The command of the Dart extractor of this tree, with the hash of its source given as a compile-time value (what `rivus setup dart` does for the
    executable), so that its `--version` says the source it is made from."""
    return "dart run -DRIVUS_SRC_HASH=%s %s" % (extractor_hash("dart"), os.path.join(ROOT, "extractor/dart/bin/rivus_dart.dart"))


def build_rivus(out):
    """rivus built from this tree like `make build` does: with the hashes of the source of the two extractors (-ldflags -X, ADR-126)."""
    flags = "-X main.rustSourceHash=%s -X main.dartSourceHash=%s" % (extractor_hash("rust"), extractor_hash("dart"))
    subprocess.run(["go", "build", "-ldflags", flags, "-o", out, "./cmd/rivus"], cwd=ROOT, check=True)


def extractor_failure(report, lang):
    """The check of the pass line on summary.extractor (ADR-126): the extractor of lang must say it was built from the source this rivus was built with
    (match "yes"). Returns None when it is, else (state, message): the state is the match, or "missing" (the report has no entry for lang: a rivus
    older than the check); the message says how to repair it. A target of Go has no extractor to check."""
    if lang not in ("rust", "dart"):
        return None
    entries = ((report or {}).get("summary") or {}).get("extractor") or []
    mine = [e for e in entries if e.get("lang") == lang]
    if not mine:
        if any(e.get("match") == "n/a(-ir)" for e in entries):
            return "n/a(-ir)", "summary.extractor says the IR was read from a file (-ir); this tool never uses -ir, so the result is not of the extractor"
        return "missing", "summary.extractor has no entry for the %s extractor (a rivus without the check of ADR-126): build rivus with `make build` from this tree" % lang
    e = mine[0]
    state, path, detail = e.get("match", ""), e.get("path", "?"), e.get("detail", "")
    again = ("`rivus setup rust -source extractor/rust` (or, in extractor/rust, RIVUS_EXTRACTOR_SRC_HASH=$(go run ./scripts/extractor-hash rust) cargo build)" if lang == "rust"
             else "`rivus setup dart -source extractor/dart` (the command of this tool does it with -DRIVUS_SRC_HASH)")
    if state == "yes":
        return None
    if state == "no":
        return state, "the %s extractor %s was built from another source than this rivus (extractor %s, rivus %s): build the extractor again from the same commit: %s" % (
            lang, path, (e.get("source_hash") or "?")[:12], (e.get("expected_hash") or "?")[:12], again)
    if state == "unknown_binary":
        return state, "this rivus was built without the hash of the source of the extractors (a plain go build), so the %s extractor %s cannot be checked: build rivus with `make build`" % (lang, path)
    if state == "unknown_extractor":
        return state, "the %s extractor %s did not say which source it was built from (%s): build it again: %s" % (lang, path, detail or "no detail", again)
    if state == "unchecked":
        return state, "the %s extractor %s was not asked which source it was built from (%s): run it where the isolation is (macOS), or build the extractor again: %s" % (lang, path, detail or "no detail", again)
    return state or "missing", "summary.extractor.match is %r, not 'yes'" % (state,)


def expand_args(args, src, subdir):
    """The flags of the target; `{root}` is the root of the tree (src without its subdir), e.g. for -allow-deps-under."""
    root = os.path.normpath(os.path.join(src, *([".."] * len([x for x in subdir.split("/") if x])))) if subdir else src
    return [a.replace("{root}", root) for a in args]


def rust_sdkroot_args(env):
    """`-isolation-env SDKROOT=<path>` for the build of a Rust target, when BENCH_RUST_SDKROOT is set (the SDK of this machine; the CLT `MacOSX27.0.sdk`
    cannot link here, docs/DESIGN-install.md). Not set (or empty): nothing is passed. The path is never written in the repository."""
    v = (env.get("BENCH_RUST_SDKROOT") or "").strip()
    return ["-isolation-env", "SDKROOT=" + v] if v else []


def analyze_once(rivus, t, src, env, limit_s, cap_bytes, work, tag):
    """One run with the default flags (t["args"]). Returns dict: wall, cpu, peak (= b, the whole process tree: the
    memory of the verdict), peak_a (the main process), peak_how, big, converged, why, report."""
    if t["lang"] == "rust":
        rustenv.require_rust_data_dir(env, "deps_bench")
    cmd = [rivus, "analyze", "-lang", t["lang"], "-C", src] + expand_args(t.get("args", []), src, t.get("subdir", "")) + (rust_sdkroot_args(env) if t["lang"] == "rust" else [])
    outp, errp = os.path.join(work, tag + ".out.json"), os.path.join(work, tag + ".err")
    with heavy_lock("deps_bench analyze " + tag):
        load = os.getloadavg()[0]
        sn0 = snapshot()
        m = run_watched(cmd, ROOT, env, limit_s, cap_bytes, errp, outp)
        note = format_snapshots(sn0, snapshot())
    rc, why, wall, peak = m["rc"], m["why"], m["wall"], m["b"]  # b: the footprint on macOS, the RSS sum otherwise (m["basis"])
    rep, converged = None, False
    if not why and rc in (0, EXIT_INCOMPLETE):  # 4: the report is written, but the analysis is not complete (timeout, not converged); converged below says it
        try:
            rep = json.load(open(outp))
            st = rep.get("stats", {})
            # a run that stopped a function on the clock (-func-timeout, -timeout) depends on the load of the machine:
            # the same run on a quieter one may find more, so it does not count as converged (stats.load_dependent)
            converged = bool(st.get("converged", False)) and not st.get("timed_out", False) and not st.get("load_dependent", False)
        except (OSError, ValueError):
            rep = None
    return {"wall": wall, "peak": peak, "converged": converged, "why": why, "report": rep, "peak_how": "tree-sum-" + m["basis"], "peak_rss": m["b_rss"], "basis": m["basis"], "peak_a": m["a"], "cpu": m["cpu"], "big": m["big"], "big_rss": m["big_rss"], "at_s": m["at_s"], "rc": rc, "load": load, "note": note}


def median_run(runs):
    """One run for n runs of the same target: the run of the median wall time, with the median wall, cpu and peak; converged only when all did.
    The notes (load, busy processes) of every run are kept."""
    if len(runs) == 1:
        return runs[0]
    by = sorted(runs, key=lambda r: r["wall"])
    r = dict(by[(len(by) - 1) // 2])
    if len({x["basis"] for x in runs}) > 1:  # one run fell back to RSS: no mix, all runs are taken in RSS
        runs = [dict(x, peak=x["peak_rss"], basis="rss") for x in runs]
        r["basis"] = "rss"
    for k in ("wall", "cpu", "peak", "peak_rss"):
        r[k] = statistics.median(x[k] for x in runs)
    r["converged"] = all(x["converged"] for x in runs)
    r["why"] = next((x["why"] for x in runs if x["why"]), "")
    r["note"] = " | ".join("run %d: %s" % (i + 1, x["note"]) for i, x in enumerate(runs))
    r["walls"] = [x["wall"] for x in runs]
    return r


def passline(name, t, rivus, cache, outdir, limit_s, cap_gb, repeat=1):
    """Returns (line, row). A Rust target first needs `sandbox-exec` to deny a write (else (line, None): nothing was built or run, no row) and gets an empty
    HOME inside the cache, removed at the end; a row whose result is `isolation_failed` is a failure (run_passline: exit status 1)."""
    if t["lang"] != "rust":
        return passline_run(name, t, rivus, cache, outdir, limit_s, cap_gb, repeat)
    ok, why = sandbox_denies_write(cache)
    if not ok:
        return "INFO %s isolation_failed: %s (before the run: nothing was built or run)" % (name, why), None
    home = tempfile.mkdtemp(prefix="home-", dir=cache)
    try:
        return passline_run(name, t, rivus, cache, outdir, limit_s, cap_gb, repeat, home)
    finally:
        shutil.rmtree(home, ignore_errors=True)


def passline_run(name, t, rivus, cache, outdir, limit_s, cap_gb, repeat=1, home=None):
    with fixed_work(cache, "passline-" + name) as work:
        env = dict(os.environ)
        if home:
            env["HOME"] = home  # empty: the data of rivus for Rust (os.UserConfigDir) and its cache are made here, not in the shared places
        cap = int(cap_gb * GB)

        def tree_for(commit):
            if "local" in t:
                return os.path.join(ROOT, t["local"])
            return fetch(name, t, commit, cache)

        def run(commit, tag):
            tree = tree_for(commit)
            src, e, err = prepare(name, t, tree, work, env)
            if err:
                raise SystemExit("deps_bench: %s: %s" % (name, err))
            r = analyze_once(rivus, t, src, e, limit_s, cap, work, tag)
            if r["rc"] not in (0, EXIT_INCOMPLETE) and not r["why"]:  # the run ended by itself with an error: its last lines of stderr are kept (they are lost with the work dir otherwise)
                errp = os.path.join(work, tag + ".err")
                if os.path.exists(errp):
                    shutil.copy(errp, os.path.join(outdir, "%s-%s.err" % (name, tag)))
                    print("deps_bench: %s exited with status %s; stderr kept in %s-%s.err (last lines: %s)" % (name, r["rc"], name, tag, open(errp, errors="replace").read()[-400:].replace("\n", " | ")), file=sys.stderr)
            if r["report"] is not None:  # the report goes to the untracked dir; counts are read from it
                shutil.copy(os.path.join(work, tag + ".out.json"), os.path.join(outdir, "%s-%s.json" % (name, tag)))
            return r

        main = median_run([run(t.get("commit", ""), "pin" if i == 0 else "pin%d" % (i + 1)) for i in range(max(1, repeat))])
        if main["report"] is not None and not main["why"]:  # the extractor and rivus must be of the same source, before any further run (ADR-126)
            bad = extractor_failure(main["report"], t["lang"])
            if bad:
                return "FAIL %s extractor_mismatch(%s): %s (no ledger row; the result is not used)" % (name, bad[0], bad[1]), None
        detected, quiet, d, through, located, score = None, None, t.get("detect"), None, None, None
        if d and d.get("mode") == "score":  # per published vulnerability, by place; the count n/N only (no finding count, list, path or place anywhere)
            truth = json.load(open(os.path.join(ROOT, d["truth"])))["sites"]
            total = len({x["vuln"] for x in truth if x.get("label") == "vuln"})
            score = score_counts(truth, main["report"]) if main["converged"] else (None, total)
        elif d and d.get("mode") == "through":  # per vulnerability; the position count beside it (information)
            truth = json.load(open(os.path.join(ROOT, d["truth"])))["sites"]
            tp, n = through_counts(truth, main["report"]) if main["converged"] else (None, len([x for x in truth if "file" in x and x.get("label") == "vuln"]))
            detected, through = detected_through(tp), (tp, n)
        elif d and d.get("mode") == "location":  # per vulnerability, by the places of the truth (the pre-fix commit)
            truth = json.load(open(os.path.join(ROOT, d["truth"])))["sites"]
            cat = d["category"]
            cnt = lambda r: count_category(r["report"], cat) if r["converged"] else None
            loc = lambda r, side: location_counts(truth, r["report"], cat, side)[0] if r["converged"] else None
            pre = run(d["prefix_commit"], "prefix")
            located = loc(pre, "prefix")
            detected = detected_located(located)
            fixed_n = fixed_loc = None
            if detected:  # the fixed commit is read only to record quiet_after_fix (not part of PASS/FAIL)
                fx = run(d["fixed_commit"], "fixed")
                fixed_n, fixed_loc = cnt(fx), loc(fx, "fixed")
                quiet = quiet_after_fix(fixed_n)
            print("deps_bench: %s %s findings of %s: pre-fix %s (at the labelled places %s), fixed %s (at the places of the fixed commit %s)" % (name, d["id"], cat, cnt(pre), located, fixed_n, fixed_loc), file=sys.stderr)
        elif d:
            cnt = lambda r: count_category(r["report"], d["category"]) if r["converged"] else None
            pre = run(d["prefix_commit"], "prefix")
            detected = detection(cnt(pre))
            fixed_n = None
            if detected:  # the fixed commit is read only to record quiet_after_fix (not part of PASS/FAIL)
                fixed_n = cnt(run(d["fixed_commit"], "fixed"))
                quiet = quiet_after_fix(fixed_n)
            print("deps_bench: %s %s findings of %s: pre-fix %s, fixed %s" % (name, d["id"], d["category"], cnt(pre), fixed_n), file=sys.stderr)
        coarsened = ((main["report"] or {}).get("stats") or {}).get("scc_coarsened", 0) if main["report"] is not None else None
        passed, body = judge(main["wall"], limit_s, main["peak"] / GB, cap_gb, main["converged"], detected, through, quiet, located, score, coarsened, t.get("expect_coarsened", 0), t.get("detect_in_verdict", True))
        decided = t.get("decided", False)
        iso_ok, iso_why, unc = True, "", None
        failed_run = main["rc"] not in (0, EXIT_INCOMPLETE) and not main["why"]  # rivus ended by itself with an error: no report, nothing to read
        if t["lang"] == "rust" and not failed_run and not main["why"]:
            iso_ok, iso_why = isolation_ok(main["report"])
            unc = uncertainty_counts(main["report"])
            body += " " + format_uncertainties(unc) + " isolation=" + ("sandbox" if iso_ok else "FAILED(%s)" % iso_why)
        label = None
        if main["why"]:
            result = "stopped_" + main["why"]
        elif failed_run:
            result = "analyze_failed"
        elif not iso_ok:
            result = "isolation_failed"
        else:
            result, label = result_labels(passed, decided, main["peak"] / GB, cap_gb)
        if result == "analyze_failed":  # a failure whose cause is not known adds no row (and no number is read from it)
            return "%s %s analyze_failed: exit status %s after %.0f s (no report; no ledger row; the stderr is kept in the output directory)" % ("INFO" if not decided else "FAIL", name, main["rc"], main["wall"]), None
        extra = [a for a in t.get("args", [])] + (["-limit %g" % limit_s] if limit_s != t["time_limit_s"] else []) + (["-cap %g" % cap_gb] if cap_gb != t["mem_cap_gb"] else [])
        extra += ["%s=%s" % (k, v) for k, v in sorted(env.items()) if k == "GOMEMLIMIT" or (k.startswith("RIVUS_") and k not in ("RIVUS_HEAVY_LOCK_HELD", "RIVUS_HEAVY_LOCK", "RIVUS_RUST_TOOLCHAIN_DIR", "RIVUS_RUST_EXTRACTOR"))]  # the two paths of Rust are the paths of this machine: not written
        if t["lang"] == "rust" and rust_sdkroot_args(env):
            extra.append("SDKROOT given (BENCH_RUST_SDKROOT)")
        if repeat > 1:
            extra.append("repeat %d (median)" % repeat)
        if unc is not None:
            extra.append("HOME=empty dir in the cache; isolation=%s; %s" % ("sandbox" if iso_ok else "FAILED", format_uncertainties(unc)))
        row = {"target": name, "target_commit": t.get("commit") or "in-tree", "kind": "passline", "settings": "%s; %s" % (" ".join(extra) or "default", main["note"]),
               "wall_s": "%.1f" % main["wall"], "cpu_s": "%.1f" % main["cpu"], "peak_tree_gb": "%.2f" % (main["peak_rss"] / GB),
               "peak_footprint_gb": "%.2f" % (main["peak"] / GB) if main["basis"] == "footprint" else None, "mem_basis": main["basis"],
               "converged": main["converged"], "detected": detected if score is None else "%s/%d" % ("-" if score[0] is None else score[0], score[1]), "quiet_after_fix": quiet, "exit_code": main["rc"],
               "load_start": "%.2f" % main["load"], "result": result}
        row.update({k: v for k, v in heavy_ledger.report_columns(main["report"]).items() if v is not None})
        if score is not None:
            row.pop("findings", None)  # the count of findings of this target is not written
        return format_line(name, decided, passed, body, label if result in ("PASS", "PASS-thin") else None), row


def git_commit():
    return subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, capture_output=True, text=True).stdout.strip()


def add_to_ledger(row, a, commit):
    """One row for a run, unless -no-ledger. date and rivus_commit are added here."""
    if a.no_ledger:
        return
    row = dict(row, date=datetime.date.today().isoformat(), rivus_commit=commit)
    heavy_ledger.append_row(row, a.ledger)


def run_passline(names, a):
    targets = json.load(open(TARGETS_FILE))
    cache, outdir = cache_dir(), out_dir()
    commit = git_commit()
    failed = []
    with tempfile.TemporaryDirectory(prefix="deps-bench-") as bw:
        rivus = a.rivus
        if not rivus:
            rivus = os.path.join(bw, "rivus")
            build_rivus(rivus)
        for name in names:
            t = targets[name]
            line, row = passline(name, t, rivus, cache, outdir, a.limit or t["time_limit_s"], a.cap or t["mem_cap_gb"], a.repeat)
            print(line, flush=True)
            if row is not None:
                add_to_ledger(row, a, commit)
            if row_failed(row):
                failed.append(name)
    if failed:
        raise SystemExit("deps_bench: no usable result (the isolation check failed before or after the run, the extractor does not match this rivus (extractor_mismatch), or rivus analyze ended with an error): %s" % ", ".join(failed))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-timeout", type=int, default=600, help="seconds (the -timeout of rivus; the group is killed GRACE s later)")
    ap.add_argument("-rivus", default="", help="the rivus binary (default: built from this tree)")
    ap.add_argument("-ledger", default=heavy_ledger.LEDGER, help="the ledger (bench/heavy-ledger.tsv)")
    ap.add_argument("-no-ledger", action="store_true", help="do not add a row (a binary with a measurement patch; a run that is not used as a number)")
    ap.add_argument("-limit", type=float, default=0, help="pass line: the time limit in seconds (default: the one of the target)")
    ap.add_argument("-cap", type=float, default=0, help="pass line: the memory cap in GiB (default: the one of the target)")
    ap.add_argument("-repeat", "--repeat", type=int, default=1, help="pass line: run each target n times; the median of the time, the CPU and the peak goes to the verdict (converged: all runs). The decision of D is the median of 3 on a quiet machine")
    ap.add_argument("benches", nargs="*", default=list(BENCHES))
    a = ap.parse_args()
    targets = json.load(open(TARGETS_FILE))
    if a.benches and all(b in targets for b in a.benches):
        return run_passline(a.benches, a)
    if any(b in targets for b in a.benches):
        raise SystemExit("deps_bench: do not mix the benches of the ledger and the targets of the pass line")
    commit = git_commit()
    with tempfile.TemporaryDirectory(prefix="deps-bench-") as work:
        rivus = a.rivus
        if not rivus:
            rivus = os.path.join(work, "rivus")
            build_rivus(rivus)
        for name in a.benches:
            with fixed_work(cache_dir(), "deps-" + name) as w:  # the copy of a Dart target is made here: the same path every run
                row, why = measure(name, rivus, a.timeout, w)
            if row is None:
                print("deps_bench: %s skipped: %s" % (name, why), file=sys.stderr)
                continue
            add_to_ledger(row, a, commit)
            print("%s %s wall=%ss mem=%sGiB(%s) result=%s" % (name, row["kind"], row["wall_s"], row.get("peak_footprint_gb") or row["peak_tree_gb"], row.get("mem_basis") or "rss", row["result"]), flush=True)


if __name__ == "__main__":
    main()
