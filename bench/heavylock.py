#!/usr/bin/env python3
"""One lock for the whole machine for the heavy runs (analyses that use several GB of memory).

Why: when several sessions run heavy analyses at the same time, the load goes into the measurement, and the
24 GB of the machine are not enough. A tool that starts such a run takes this lock first and waits when another
one holds it.

  from heavylock import heavy_lock
  with heavy_lock("passline memos"): ...

  python3 bench/heavylock.py [--reason TEXT] -- <command...>     (for shell tools; the command replaces this process)

- The file: $RIVUS_HEAVY_LOCK, else ~/.cache/rivus/heavy-run.lock. A fixed place per user (not the temporary
  directory: the sessions and the sandboxes may have different ones), outside the repository, the same for
  every work tree.
- fcntl.flock: the OS releases it when the process ends, so no stale lock is left. There is no way to force it
  off; end the process that holds it (its PID and command are written in the file, and printed by the waiter).
- The wait is limited: $RIVUS_HEAVY_LOCK_WAIT_S seconds (default 1800). After that it fails (exit 75), with one
  line of the reason, and leaves no process.
- Re-entry: a process that holds the lock sets RIVUS_HEAVY_LOCK_HELD=1 for its children, so a tool that is
  started by a tool that holds the lock does not wait for itself.
"""
import contextlib
import fcntl
import os
import subprocess
import sys
import time

HELD = "RIVUS_HEAVY_LOCK_HELD"
DEFAULT_WAIT_S = 1800.0
EXIT_TIMEOUT = 75


BUSY_PCT = 20.0
BUSY_TOP = 5


def busy_processes(min_pct=BUSY_PCT, top=BUSY_TOP):
    """The command names (no arguments, no path) of the processes above min_pct of one CPU, the top ones first. [] when ps fails.
    The lock does not stop a run that does not take it (a build, another session), so a quiet run needs this record."""
    try:
        out = subprocess.run(["ps", "-axo", "pcpu=,comm="], capture_output=True, text=True, timeout=10).stdout
    except (OSError, subprocess.SubprocessError):
        return []
    rows = []
    for l in out.splitlines():
        a, _, comm = l.strip().partition(" ")
        try:
            pct = float(a)
        except ValueError:
            continue
        if pct >= min_pct and comm.strip():
            rows.append((pct, os.path.basename(comm.strip())))
    rows.sort(key=lambda x: -x[0])
    return [n for _, n in rows[:top]]


def snapshot():
    """The load average (1, 5, 15 minutes) and the busy processes now."""
    try:
        load = os.getloadavg()
    except OSError:
        load = ()
    return {"load": load, "busy": busy_processes()}


def format_snapshots(start, end):
    """One text for the settings column: the load at the start and at the end, and the busy processes of both (names only)."""
    f = lambda sn: "/".join("%.2f" % x for x in sn["load"]) or "-"
    g = lambda sn: ",".join(sn["busy"]) or "none"
    return "load start %s end %s (1/5/15 min); busy(>=%d%% cpu) start: %s end: %s" % (f(start), f(end), BUSY_PCT, g(start), g(end))


class HeavyLockTimeout(Exception):
    pass


def lock_path():
    p = os.environ.get("RIVUS_HEAVY_LOCK")
    return p if p else os.path.join(os.path.expanduser("~"), ".cache", "rivus", "heavy-run.lock")


def wait_limit():
    try:
        return float(os.environ.get("RIVUS_HEAVY_LOCK_WAIT_S", DEFAULT_WAIT_S))
    except ValueError:
        return DEFAULT_WAIT_S


def _holder(path):
    try:
        with open(path) as f:
            line = f.read().strip().split("\n")[0]
    except OSError:
        return "unknown"
    pid, _, rest = line.partition("\t")
    return "PID %s, %s" % (pid, rest) if pid else "unknown"


def _acquire(reason, cmd, poll):
    """Opens and locks the file; returns the open file (the lock lives as long as it is open)."""
    path = lock_path()
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    f = open(path, "a+")
    limit, start, told = wait_limit(), time.monotonic(), False
    while True:
        try:
            fcntl.flock(f, fcntl.LOCK_EX | fcntl.LOCK_NB)
            break
        except OSError:
            if not told:
                print("別の重い実行を待っています(ロックを持つプロセス: %s)" % _holder(path), file=sys.stderr, flush=True)
                told = True
            if time.monotonic() - start >= limit:
                f.close()
                raise HeavyLockTimeout("heavylock: gave up after %.0f s waiting for the heavy-run lock %s (held by: %s); RIVUS_HEAVY_LOCK_WAIT_S changes the limit" % (limit, path, _holder(path)))
            time.sleep(poll)
    f.seek(0)
    f.truncate()
    f.write("%d\t%s%s\n" % (os.getpid(), cmd, (" [" + reason + "]") if reason else ""))
    f.flush()
    return f


@contextlib.contextmanager
def heavy_lock(reason="", poll=0.5):
    """Takes the machine-wide lock for the block (no-op when a parent process holds it). Raises HeavyLockTimeout."""
    if os.environ.get(HELD) == "1":
        yield
        return
    f = _acquire(reason, " ".join(sys.argv)[:200], poll)
    os.environ[HELD] = "1"
    try:
        yield
    finally:
        del os.environ[HELD]
        f.close()


def main(argv):
    reason = ""
    if argv and argv[0] == "--reason":
        reason, argv = argv[1], argv[2:]
    if argv and argv[0] == "--":
        argv = argv[1:]
    if not argv:
        print("usage: heavylock.py [--reason TEXT] -- <command...>", file=sys.stderr)
        return 64
    if os.environ.get(HELD) != "1":
        try:
            f = _acquire(reason, " ".join(argv)[:200], 0.5)
        except HeavyLockTimeout as e:
            print(e, file=sys.stderr)
            return EXIT_TIMEOUT
        os.set_inheritable(f.fileno(), True)  # the command keeps the lock (also when it outlives this process)
        os.environ[HELD] = "1"
    try:
        os.execvp(argv[0], argv)
    except OSError as e:
        print("heavylock: cannot run %s: %s" % (argv[0], e), file=sys.stderr)
        return 127


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
