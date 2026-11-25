#!/usr/bin/env python3
"""Compare the graph representation of two versions of rivus on one target (the engine of bench/compare.sh --graph-check).

  graph_check.py -name NAME -base-bin B -head-bin H -graphcheck-bin G -out DIR (-dir D [-rules F] [-flags "..."] | -target T)
                  [-base-workers N -head-workers M]   (the self-check: the same binary as base and head, run with N and M workers)
                  [-lang dart -base-extractor CMD -head-extractor CMD]   (a Dart directory: the command of the extractor of each side)

A change for memory or cost must not change what the analysis builds (docs/WORKFLOW.md). On the same code, each version gives:
  IR         `rivus debug extract -all`            (compared by accept.Compare, the comparison of the acceptance test)
             Dart: the output of the extractor of that side, the command that `rivus index` and `rivus analyze` run (-base-extractor, -head-extractor)
  edges      `rivus index` (the graph file)        (every edge of the call graph with its call site, kind and number of candidates)
  summaries  RIVUS_INTERNAL_SUMMARY_DUMP of `rivus analyze` (one line for each function: the hash of the whole summary)
  findings   the `findings` and `uncertainties` of the same `rivus analyze` (the set: the order of the array does not count)
  findings-order, uncertainties-order
             the same two arrays in the order of the output (SHA-256 of the array with the keys of each object sorted and nothing else
             changed: the steps of a path and the sentence of an uncertainty are in it). A change of the order is a change for the user.
and prints one line for each: "same" or "DIFFERENT" with how many and the first few function IDs. A version that does not write the
dump (one from before this check) leaves "summaries" as NOT COMPARED; it is not a pass.

-dir: a directory (the benchmarks of the tree, a synthetic one, a clone); -target: a target of bench/deps-targets.json (memos), fetched
and run like the pass line (the process tree is watched against the memory cap of the target; the heavy-work lock is held for each run).
Go and Dart (-lang dart, with -dir; a target of bench/deps-targets.json is Go). Rust is not covered. Exit status: 0 everything compared is the
same (and summaries were compared); 1 a difference; 2 a run failed; 3 same, but a part was not compared; 4 same, and the only thing missing is that an analysis did not converge (see below). Standard library only.
A run of `rivus analyze` that ends with exit code 4 (the analysis is not complete: the pass limit or the timeout; zoo-dart has cells that stop by the
pass limit on purpose; zoo-go has cells that do not converge too) is compared as far as it goes, and the line "convergence: NOT CONVERGED" makes the exit status 4
when everything else is the same (a part not compared gives 3, a difference 1, a failed run 2): a result that stopped is not a pass, but a caller that knows
the target stops can tell it from a part that was not compared. A clock never gives 4 or 3 (clock_line is a line only).
"""
import argparse, collections, hashlib, json, os, shlex, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import deps_bench as db  # noqa: E402
from heavylock import HeavyLockTimeout  # noqa: E402

DUMP_ENV = "RIVUS_INTERNAL_SUMMARY_DUMP"


def patterns_of(flags, loose=False):
    """The packages in a list of flags: ./... and ./pkg; loose (the flags of a benchmark of compare.sh, which has no flag with a value):
    every word that is not a flag, an import path such as net/http too. A target of deps-targets.json has flags with values (-max-memory 4GiB)."""
    return [f for f in flags if (not f.startswith("-") if loose else f.startswith("./"))]


# A comparison must not depend on a clock: a function that is stopped by -func-timeout (30 s by default) leaves a func_timeout
# uncertainty and a summary that stopped in the middle, and which functions that are depends on the load and on the number of
# workers (on a slower machine, or with more workers than cores, a different one is stopped). The runs of the check get a limit
# that is out of reach, and a run that was stopped by a clock anyway is said so (clock_line; a line, not a failure).
FUNC_TIMEOUT = "24h"


def run_side(side, binary, src, rules, flags, env_base, work, limit_s, cap, lock_reason, loose=False, workers="", extractor=""):
    """Runs extract, index and analyze of one version. Returns (files, error, incomplete): incomplete is True when analyze ended with exit code 4.
    extractor: the command of the Dart extractor of this version (empty: Go)."""
    pats = patterns_of(flags, loose)
    files = {"ir": os.path.join(work, side + ".ir.jsonl"), "db": os.path.join(work, side + ".graph.db"),
             "dump": os.path.join(work, side + ".summaries.txt"), "report": os.path.join(work, side + ".report.json")}
    rules_args = ["-rules", rules] if rules else []
    if workers:  # -workers N: the same result with any number of them (the self-check: the hash does not hold the order of the work)
        flags = ["-workers", workers] + flags  # before the packages: the flags stop at the first one
    if extractor:  # Dart: the IR is the output of the extractor (the way compare.sh makes it); index and analyze run the same command
        steps = [
            ("extract", shlex.split(extractor) + [src], files["ir"], {}),
            ("index", [binary, "index", "-lang", "dart", "-C", src, "-graph", files["db"]] + rules_args + flags, os.path.join(work, side + ".index.out"), {"RIVUS_DART_EXTRACTOR": extractor}),
            ("analyze", [binary, "analyze", "-lang", "dart", "-C", src, "-func-timeout", FUNC_TIMEOUT] + rules_args + flags, files["report"], {DUMP_ENV: files["dump"], "RIVUS_DART_EXTRACTOR": extractor}),
        ]
    else:
        steps = [
            ("extract", [binary, "debug", "extract", "-all", "-C", src] + pats, files["ir"], {}),
            ("index", [binary, "index", "-C", src, "-graph", files["db"]] + rules_args + flags, os.path.join(work, side + ".index.out"), {}),
            ("analyze", [binary, "analyze", "-C", src, "-func-timeout", FUNC_TIMEOUT] + rules_args + flags, files["report"], {DUMP_ENV: files["dump"]}),
        ]
    incomplete = False
    for name, cmd, out, extra in steps:
        env = {k: v for k, v in env_base.items() if k != DUMP_ENV}
        env.update(extra)
        try:
            with db.heavy_lock("graph_check %s %s" % (side, name)):
                try:
                    m = db.run_watched(cmd, src, env, limit_s, cap, os.path.join(work, "%s.%s.err" % (side, name)), out)
                except OSError as e:  # the command does not exist (a Dart extractor that is not there)
                    return files, "%s %s could not start (%s)" % (side, name, e), incomplete
        except HeavyLockTimeout as e:  # the machine is busy: not run (exit 2 with a line), not a traceback
            return files, "%s %s did not run: %s" % (side, name, e), incomplete
        if m["why"]:
            return files, "%s %s stopped (%s)" % (side, name, m["why"]), incomplete
        ok = (0, db.EXIT_INCOMPLETE) if name == "analyze" else (0,)
        if m["rc"] not in ok:
            return files, "%s %s failed (exit code %s; %s)" % (side, name, m["rc"], os.path.join(work, "%s.%s.err" % (side, name))), incomplete
        if name == "analyze" and m["rc"] == db.EXIT_INCOMPLETE:
            incomplete = True
    return files, None, incomplete


def canon_items(path, key):
    try:
        rep = json.load(open(path))
    except (OSError, ValueError) as e:
        return None, str(e)
    return collections.Counter(json.dumps(x, sort_keys=True) for x in (rep.get(key) or [])), None


def compare_items(what, a, b):
    """Findings or uncertainties: the same multiset or not. Returns (line, same)."""
    ca, ea = canon_items(a, what)
    cb, eb = canon_items(b, what)
    if ca is None or cb is None:
        return "%s: NOT RUN (%s)" % (what, ea or eb), False
    if ca == cb:
        return "%s: same (%d)" % (what, sum(ca.values())), True
    return "%s: DIFFERENT (only in the first: %d, only in the second: %d)" % (what, sum((ca - cb).values()), sum((cb - ca).values())), False


def order_hash(path, key):
    """SHA-256 of the array `key` of the report with the order of the array kept and only the keys of each object sorted."""
    try:
        rep = json.load(open(path))
    except (OSError, ValueError) as e:
        return None, None, str(e)
    items = rep.get(key) or []
    return hashlib.sha256(json.dumps(items, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest(), len(items), None


def compare_order(what, a, b):
    """The array in the order of the output: the same hash or not. Returns (line, same)."""
    ha, na, ea = order_hash(a, what)
    hb, nb, eb = order_hash(b, what)
    name = what + "-order"
    if ha is None or hb is None:
        return "%s: NOT RUN (%s)" % (name, ea or eb), False
    if ha == hb:
        return "%s: same (%d, sha256 %s)" % (name, na, ha[:12]), True
    return "%s: DIFFERENT (%d in the first, %d in the second; sha256 %s and %s)" % (name, na, nb, ha[:12], hb[:12]), False


def graphcheck(gc, kind, a, b):
    p = subprocess.run([gc, kind, a, b], capture_output=True, text=True)
    if p.returncode not in (0, 1):
        return "%s: NOT RUN (%s)" % (kind, p.stderr.strip()[-200:]), 2
    return p.stdout.rstrip("\n"), p.returncode


def worse(status, rc):
    """The exit status of the whole: a failed run (2) over a difference (1) over a part not compared (3) over only a run that did not converge (4) over the same (0)."""
    rank = {0: 0, 4: 1, 3: 2, 1: 3, 2: 4}
    return rc if rank[rc] > rank[status] else status


def convergence_line(base_incomplete, head_incomplete):
    """The line for a run that stopped (exit code 4), or (None, 0). rc 4: a result that stopped is not a pass (exit 4 of this tool: same, only the convergence is missing)."""
    who = " and ".join(n for n, f in (("base", base_incomplete), ("head", head_incomplete)) if f)
    if not who:
        return None, 0
    return "convergence: NOT CONVERGED (analyze of %s ended with exit code 4: the analysis is not complete; the lines above compare what it had when it stopped)" % who, 4


def coarsened(path):
    """stats.scc_coarsened of a report (0 when it has none), or None when the report cannot be read."""
    try:
        return int((json.load(open(path)).get("stats") or {}).get("scc_coarsened", 0))
    except (OSError, ValueError, TypeError):
        return None


def coarsen_line(base_n, head_n):
    """The line for the number of components whose dependency functions are solved at depth 0 (-scc-coarsen, ADR-124). A target
    that is not meant to be hit has 0 on both sides (same). A target that is hit has a difference of the kind the line says: it is
    classified, not a regression. Returns (line, status); status 0 when both are 0 or equal, else 1."""
    if base_n is None or head_n is None:
        return "scc_coarsened: NOT COMPARED (a report cannot be read)", 3
    if base_n == head_n == 0:
        return "scc_coarsened: same (0: no component is large)", 0
    if base_n == head_n:
        return "scc_coarsened: same (%d)" % head_n, 0
    return "scc_coarsened: DIFFERENT (base %d, head %d: the components that are solved at depth 0 changed; the differences of this target are classified, not a regression)" % (base_n, head_n), 1


def clock_line(base_report, head_report):
    """The signal of the clock, apart from the comparison (it never changes the exit status): for each run, how many functions a time
    limit stopped (stats.func_timeouts, run_timeouts, timed_out) and which (the func_timeout and run_timeout uncertainties name them; the
    report has no time for each function). The line is always there; it starts with ::warning:: (a GitHub annotation) when a run has any."""
    parts, any_stopped = [], False
    for n, path in (("base", base_report), ("head", head_report)):
        try:
            rep = json.load(open(path))
        except (OSError, ValueError):
            parts.append("%s: no report" % n)
            continue
        st = rep.get("stats") or {}
        names = sorted({u.get("func") or "?" for u in (rep.get("uncertainties") or []) if (u.get("detail") or "").startswith(("func_timeout", "run_timeout"))})
        cnt = (st.get("func_timeouts") or 0) + (st.get("run_timeouts") or 0) + (1 if st.get("timed_out") else 0)
        any_stopped = any_stopped or cnt > 0 or bool(st.get("load_dependent"))
        parts.append("%s %d (run_timeouts %d, timed_out %s; %s)" % (n, st.get("func_timeouts") or 0, st.get("run_timeouts") or 0,
                                                                                    bool(st.get("timed_out")), ", ".join(names[:10]) or "none"))
    head = "::warning::func_timeouts: a time limit stopped the analysis, so a difference may come from the machine, not the code: " if any_stopped else "func_timeouts: "
    return head + "; ".join(parts), 0


def compare(a, base, head, incomplete=(False, False)):
    lines, status = [], 0  # status: 0 same, 1 different, 2 failed, 3 not all compared, 4 same but an analysis did not converge
    for kind, key in (("ir", "ir"), ("edges", "db")):
        out, rc = graphcheck(a.graphcheck_bin, kind, base[key], head[key])
        lines.append(out)
        status = worse(status, rc)
    miss = [s for s, f in (("base", base), ("head", head)) if not os.path.exists(f["dump"])]
    if miss:
        lines.append("summaries: NOT COMPARED (%s does not write the dump: a version from before the check)" % " and ".join(miss))
        status = worse(status, 3)
    else:
        out, rc = graphcheck(a.graphcheck_bin, "summaries", base["dump"], head["dump"])
        lines.append(out)
        status = worse(status, rc)
    for what in ("findings", "uncertainties"):
        line, same = compare_items(what, base["report"], head["report"])
        lines.append(line)
        if not same:
            status = worse(status, 1 if "DIFFERENT" in line else 2)
    for what in ("findings", "uncertainties"):
        line, same = compare_order(what, base["report"], head["report"])
        lines.append(line)
        if not same:
            status = worse(status, 1 if "DIFFERENT" in line else 2)
    line, rc = coarsen_line(coarsened(base["report"]), coarsened(head["report"]))
    lines.append(line)
    status = worse(status, rc)
    lines.append(clock_line(base["report"], head["report"])[0])
    line, rc = convergence_line(*incomplete)
    if line:
        lines.append(line)
        status = worse(status, rc)
    return lines, status


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-name", required=True)
    ap.add_argument("-base-bin", required=True)
    ap.add_argument("-head-bin", required=True)
    ap.add_argument("-graphcheck-bin", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-dir")
    ap.add_argument("-rules", default="")
    ap.add_argument("-flags", default="")
    ap.add_argument("-target")
    ap.add_argument("-lang", default="go", choices=("go", "dart"), help="the language of -dir (a Rust project is not covered)")
    ap.add_argument("-base-extractor", default="", help="-lang dart: the command of the Dart extractor of the base version (e.g. 'dart run /path/extractor/dart/bin/rivus_dart.dart')")
    ap.add_argument("-head-extractor", default="")
    ap.add_argument("-base-workers", default="", help="-workers of the base run (the self-check: the same binary twice with 1 and 8)")
    ap.add_argument("-head-workers", default="")
    ap.add_argument("-limit", type=float, default=3600, help="seconds for one run")
    ap.add_argument("-cap", type=float, default=0, help="GB for the process tree (default: 8, or the cap of the target)")
    a = ap.parse_args()
    if bool(a.dir) == bool(a.target):
        ap.error("exactly one of -dir and -target")
    if a.lang == "dart" and (not a.dir or not a.base_extractor or not a.head_extractor):
        ap.error("-lang dart needs -dir, -base-extractor and -head-extractor")
    os.makedirs(a.out, exist_ok=True)
    env = dict(os.environ)
    with tempfile.TemporaryDirectory(prefix="graphcheck-") as work:
        if a.target:
            t = json.load(open(db.TARGETS_FILE))[a.target]
            if t["lang"] != "go":
                print("%s: graph check is for Go only" % a.name, file=sys.stderr)
                return 2
            tree = db.fetch(a.target, t, t["commit"], db.cache_dir())
            src, env, err = db.prepare(a.target, t, tree, work, env)
            if err:
                print("%s: %s" % (a.name, err), file=sys.stderr)
                return 2
            flags, rules = db.expand_args(t.get("args", []), src, t.get("subdir", "")), ""
            cap = int((a.cap or t["mem_cap_gb"]) * db.GB)
        else:
            src, rules, flags = os.path.abspath(a.dir), os.path.abspath(a.rules) if a.rules else "", shlex.split(a.flags)
            if a.lang == "go" and not patterns_of(flags, True):
                flags = flags + ["./..."]
            cap = int((a.cap or 8) * db.GB)
        sides, incomplete = {}, []
        for side, binary in (("base", a.base_bin), ("head", a.head_bin)):
            files, err, inc = run_side(side, os.path.abspath(binary), src, rules, flags, env, work, a.limit, cap, a.name, loose=bool(a.dir), workers=a.base_workers if side == "base" else a.head_workers,
                                       extractor=(a.base_extractor if side == "base" else a.head_extractor) if a.lang == "dart" else "")
            incomplete.append(inc)
            if err:
                line = "%s: NOT RUN (%s)" % (a.name, err)
                print(line)
                open(os.path.join(a.out, "graphcheck-%s.txt" % a.name), "w").write(line + "\n")
                return 2
            sides[side] = files
        lines, status = compare(a, sides["base"], sides["head"], tuple(incomplete))
    text = "\n".join("%s %s" % (a.name, l.replace("\n", "\n  ")) for l in lines) + "\n"
    print(text, end="")
    open(os.path.join(a.out, "graphcheck-%s.txt" % a.name), "w").write(text)
    return status


if __name__ == "__main__":
    sys.exit(main())
