#!/usr/bin/env python3
"""The one ledger of the heavy measurements: bench/heavy-ledger.tsv. One row per run, for the pass line
(`make bench-passline`) and for the dependency benches (`make bench-deps`). Only the tools write it (bench/deps_bench.py
calls append_row); nobody edits it by hand, and the numbers are not copied to other files (point to the row: date and
commit). The notes beside it are in bench/heavy-ledger.md. A line that starts with `#` is a comment (skipped).

Read it (read only):

  python3 bench/heavy_ledger.py [--kind passline|deps] [--target memos] [--last N] [--ledger FILE]

The margin is not a column. The table computes it: memory_limit_bytes - memory_peak_bytes, and, when the run recorded
it, memory_limit_bytes - live_heap_max_bytes (live_margin_gb). live_heap_max_bytes is the largest amount that the GC marked
live: a guide on the upper side (in the summary phase it is larger than what is really live), not the real live size, so
live_margin_gb is on the low side. A run of an older binary has no summary.memory_* and no gc_cpu_fraction;
those cells are empty and nothing stops.
"""
import argparse
import fcntl
import os
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LEDGER = os.path.join(ROOT, "bench", "heavy-ledger.tsv")
COLUMNS = ["date", "rivus_commit", "target", "target_commit", "kind", "settings", "wall_s", "cpu_s", "peak_tree_gb",
           "memory_limit_bytes", "memory_limit_source", "memory_peak_bytes", "live_heap_max_bytes", "gc_cpu_fraction",
           "converged", "detected", "quiet_after_fix", "findings", "func_timeouts", "budget_exhausted", "exit_code",
           "incomplete_reasons", "load_start", "result"]
GB = 1024 ** 3


def _cell(v):
    if v is None:
        return ""
    if isinstance(v, bool):
        return "yes" if v else "no"
    if isinstance(v, float):
        return ("%.4f" % v).rstrip("0").rstrip(".") if v != int(v) else str(int(v))
    return str(v).replace("\t", " ").replace("\n", " ")


def format_row(row):
    """One line (no newline) in the order of COLUMNS. A missing key is an empty cell; an unknown key is an error."""
    extra = set(row) - set(COLUMNS)
    if extra:
        raise ValueError("heavy_ledger: unknown column(s): %s" % ", ".join(sorted(extra)))
    return "\t".join(_cell(row.get(c)) for c in COLUMNS)


def append_row(row, path=LEDGER):
    """Adds one row. The file is locked (flock on itself) for the check of the header and the write, and the write is
    one os.write on an O_APPEND descriptor, so two runs at the same time cannot cut each other's line."""
    line = format_row(row) + "\n"
    fd = os.open(path, os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o644)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX)
        data = line
        if os.fstat(fd).st_size == 0:
            data = "\t".join(COLUMNS) + "\n" + line
        os.write(fd, data.encode())
    finally:
        os.close(fd)  # the lock goes with the descriptor


def _num(rep, *path):
    """A number of a report by the keys, or None when it is not there (an older binary)."""
    v = rep
    for k in path:
        if not isinstance(v, dict) or k not in v:
            return None
        v = v[k]
    return v if isinstance(v, (int, float)) and not isinstance(v, bool) else None


def report_columns(rep):
    """The columns that come from a report (the JSON of `rivus analyze`); {} for None. Never raises on a missing field."""
    if not isinstance(rep, dict):
        return {}
    s = rep.get("summary") if isinstance(rep.get("summary"), dict) else {}
    st = rep.get("stats") if isinstance(rep.get("stats"), dict) else {}
    out = {
        "memory_limit_bytes": s.get("memory_limit_bytes"),
        "memory_limit_source": s.get("memory_limit_source"),
        "memory_peak_bytes": s.get("memory_peak_bytes"),
        "live_heap_max_bytes": s.get("live_heap_max_bytes", s.get("memory_live_heap_max_bytes")),
        "gc_cpu_fraction": s.get("gc_cpu_fraction"),
        "findings": len(rep["findings"]) if isinstance(rep.get("findings"), list) else None,
        "func_timeouts": st.get("func_timeouts", 0) if "converged" in st else None,
        "budget_exhausted": st.get("budget_exhausted", 0) if "converged" in st else None,
    }
    r = s.get("incomplete_reasons")
    out["incomplete_reasons"] = ",".join(r) if isinstance(r, list) else None
    return out


def read_rows(path=LEDGER):
    """[dict] of the file; skips blank lines and `#` lines; the header is the first other line."""
    rows, header = [], None
    with open(path) as f:
        for l in f:
            l = l.rstrip("\n")
            if not l.strip() or l.startswith("#"):
                continue
            cells = l.split("\t")
            if header is None:
                header = cells
                continue
            rows.append({h: (cells[i] if i < len(cells) else "") for i, h in enumerate(header)})
    return rows


def margins(row):
    """(limit - memory_peak_bytes, limit - live_heap_max_bytes) in bytes; None where a number is missing.
    live_heap_max_bytes is what the GC marked live (it is larger than the real live size in a fast-allocating phase),
    so the second value is a low-side margin, not the real one."""
    def num(k):
        try:
            return int(float(row.get(k, "")))
        except ValueError:
            return None
    lim = num("memory_limit_bytes")
    peak, live = num("memory_peak_bytes"), num("live_heap_max_bytes")
    return (None if lim is None or peak is None else lim - peak, None if lim is None or live is None else lim - live)


def _gb(b):
    return "-" if b is None else "%.2f" % (b / GB)


def render(rows):
    head = ["date", "rivus", "target", "kind", "settings", "wall_s", "cpu_s", "tree_gb", "limit_gb", "peak_gb",
            "margin_gb", "live_margin_gb", "gc", "conv", "det", "quiet", "find", "exit", "result"]
    table = [head]
    for r in rows:
        m_peak, m_live = margins(r)
        try:
            lim = _gb(int(float(r["memory_limit_bytes"])))
        except (KeyError, ValueError):
            lim = "-"
        try:
            peak = _gb(int(float(r["memory_peak_bytes"])))
        except (KeyError, ValueError):
            peak = "-"
        v = lambda k: r.get(k) or "-"
        table.append([v("date"), v("rivus_commit"), v("target"), v("kind"), v("settings"), v("wall_s"), v("cpu_s"),
                      v("peak_tree_gb"), lim, peak, _gb(m_peak), _gb(m_live), v("gc_cpu_fraction"), v("converged"),
                      v("detected"), v("quiet_after_fix"), v("findings"), v("exit_code"), v("result")])
    w = [max(len(row[i]) for row in table) for i in range(len(head))]
    return "\n".join("  ".join(c.ljust(w[i]) for i, c in enumerate(row)).rstrip() for row in table)


def main(argv=None):
    ap = argparse.ArgumentParser(description="Print the heavy ledger with the margins (read only).")
    ap.add_argument("--kind", choices=["passline", "deps"])
    ap.add_argument("--target")
    ap.add_argument("--last", type=int, default=0, help="only the last N rows after the other filters")
    ap.add_argument("--ledger", default=LEDGER)
    a = ap.parse_args(argv)
    if not os.path.exists(a.ledger):
        print("heavy_ledger: no ledger yet (%s)" % a.ledger)
        return 0
    rows = [r for r in read_rows(a.ledger) if (not a.kind or r.get("kind") == a.kind) and (not a.target or r.get("target") == a.target)]
    if a.last > 0:
        rows = rows[-a.last:]
    print(render(rows))
    return 0


if __name__ == "__main__":
    sys.exit(main())
