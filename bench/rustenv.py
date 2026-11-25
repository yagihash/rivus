"""The data directory of rivus for Rust: a tool of bench/ that runs Rust stops when this would be the shared place of the user (issue #227).

rivus makes its Rust data (the place of `cargo fetch`, the extractor that `rivus setup rust` built) in RIVUS_RUST_DATA_DIR, else in
os.UserConfigDir()/rivus/rust (darwin: ~/Library/Application Support/rivus/rust). A run of a worker must write nothing there, so a tool names a
directory outside it (RIVUS_RUST_DATA_DIR=~/tmp/...) or makes HOME an empty directory (the default is then under that HOME). The normal
`rivus analyze` of a user is not changed: only the tools of bench/ ask for this."""
import os
import pwd
import sys


def shared_dirs(real_home=None, platform=None):
    """The place of the user that a run must not write: <real home>/Library/Application Support/rivus (darwin), <real home>/.config/rivus (others)."""
    home = real_home or pwd.getpwuid(os.getuid()).pw_dir
    sub = ("Library", "Application Support", "rivus") if (platform or sys.platform) == "darwin" else (".config", "rivus")
    return [os.path.realpath(os.path.join(home, *sub))]


def _under(path, base):
    return path == base or path.startswith(base + os.sep)


def require_rust_data_dir(env, who, real_home=None, platform=None):
    """Raises SystemExit when a run of Rust with env would use the shared place: RIVUS_RUST_DATA_DIR is not set (and HOME is the real one), or it
    is a place under the shared directory. An env whose HOME is another directory (an empty home made by the tool) is fine without it."""
    plat = platform or sys.platform
    real = real_home or pwd.getpwuid(os.getuid()).pw_dir
    given = env.get("RIVUS_RUST_DATA_DIR", "")
    if given:
        effective = given
    else:
        home = env.get("HOME") or real
        sub = ("Library", "Application Support", "rivus", "rust") if plat == "darwin" else (".config", "rivus", "rust")
        effective = os.path.join(home, *sub)
    path = os.path.realpath(os.path.expanduser(effective))
    for base in shared_dirs(real, plat):
        if _under(path, base):
            raise SystemExit(
                "%s: a run of Rust would use the shared place of rivus (%s). It writes there (cargo fetch, the built extractor); a worker must not. "
                "Point it outside: RIVUS_RUST_DATA_DIR=~/tmp/<dir> (or make HOME an empty directory)" % (who, path))
