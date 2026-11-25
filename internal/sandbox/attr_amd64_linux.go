//go:build linux && amd64

package sandbox

import "golang.org/x/sys/unix"

// attrCalls are the system calls that change the mode, the owner, the times or the extended attributes of a file: Landlock does not stop them. Each
// says where its arguments are (-1: it has none of that kind): fd is a file descriptor, dirfd the directory that path is relative to, path a string.
var attrCalls = []attrCall{
	{unix.SYS_CHMOD, "chmod", -1, -1, 0},
	{unix.SYS_FCHMOD, "fchmod", 0, -1, -1},
	{unix.SYS_FCHMODAT, "fchmodat", -1, 0, 1},
	{unix.SYS_FCHMODAT2, "fchmodat2", -1, 0, 1},
	{unix.SYS_CHOWN, "chown", -1, -1, 0},
	{unix.SYS_LCHOWN, "lchown", -1, -1, 0},
	{unix.SYS_FCHOWN, "fchown", 0, -1, -1},
	{unix.SYS_FCHOWNAT, "fchownat", -1, 0, 1},
	{unix.SYS_UTIME, "utime", -1, -1, 0},
	{unix.SYS_UTIMES, "utimes", -1, -1, 0},
	{unix.SYS_FUTIMESAT, "futimesat", -1, 0, 1},
	{unix.SYS_UTIMENSAT, "utimensat", -1, 0, 1},
	{unix.SYS_SETXATTR, "setxattr", -1, -1, 0},
	{unix.SYS_LSETXATTR, "lsetxattr", -1, -1, 0},
	{unix.SYS_FSETXATTR, "fsetxattr", 0, -1, -1},
	{unix.SYS_REMOVEXATTR, "removexattr", -1, -1, 0},
	{unix.SYS_LREMOVEXATTR, "lremovexattr", -1, -1, 0},
	{unix.SYS_FREMOVEXATTR, "fremovexattr", 0, -1, -1},
}
