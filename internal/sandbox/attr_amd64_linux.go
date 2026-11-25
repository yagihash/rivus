//go:build linux && amd64

package sandbox

import "golang.org/x/sys/unix"

// attrCalls are the system calls that change the mode, the owner, the times or the extended attributes of a file: Landlock does not stop them.
var attrCalls = []attrCall{
	byPath(unix.SYS_CHMOD, "chmod", false),
	byFd(unix.SYS_FCHMOD, "fchmod"),
	byDir(unix.SYS_FCHMODAT, "fchmodat", -1),
	byDir(unix.SYS_FCHMODAT2, "fchmodat2", 3),
	byPath(unix.SYS_CHOWN, "chown", false),
	byPath(unix.SYS_LCHOWN, "lchown", true),
	byFd(unix.SYS_FCHOWN, "fchown"),
	byDir(unix.SYS_FCHOWNAT, "fchownat", 4),
	byPath(unix.SYS_UTIME, "utime", false),
	byPath(unix.SYS_UTIMES, "utimes", false),
	byDir(unix.SYS_FUTIMESAT, "futimesat", -1),
	byDir(unix.SYS_UTIMENSAT, "utimensat", 3),
	byPath(unix.SYS_SETXATTR, "setxattr", false),
	byPath(unix.SYS_LSETXATTR, "lsetxattr", true),
	byFd(unix.SYS_FSETXATTR, "fsetxattr"),
	byPath(unix.SYS_REMOVEXATTR, "removexattr", false),
	byPath(unix.SYS_LREMOVEXATTR, "lremovexattr", true),
	byFd(unix.SYS_FREMOVEXATTR, "fremovexattr"),
	// the newer ones, that Landlock does not mediate either: setxattrat and removexattrat (Linux 6.13: dirfd, path, flags), file_setattr (6.17: dirfd, path, attr, size, flags)
	byDir(unix.SYS_SETXATTRAT, "setxattrat", 2),
	byDir(unix.SYS_REMOVEXATTRAT, "removexattrat", 2),
	byDir(unix.SYS_FILE_SETATTR, "file_setattr", 4),
}
