//go:build linux && arm64

package sandbox

import "golang.org/x/sys/unix"

// attrCalls: see attr_amd64_linux.go (arm64 has only the forms that take a descriptor or a directory, and the ones of the extended attributes).
var attrCalls = []attrCall{
	byFd(unix.SYS_FCHMOD, "fchmod"),
	byDir(unix.SYS_FCHMODAT, "fchmodat", -1),
	byDir(unix.SYS_FCHMODAT2, "fchmodat2", 3),
	byFd(unix.SYS_FCHOWN, "fchown"),
	byDir(unix.SYS_FCHOWNAT, "fchownat", 4),
	byDir(unix.SYS_UTIMENSAT, "utimensat", 3),
	byPath(unix.SYS_SETXATTR, "setxattr", false),
	byPath(unix.SYS_LSETXATTR, "lsetxattr", true),
	byFd(unix.SYS_FSETXATTR, "fsetxattr"),
	byPath(unix.SYS_REMOVEXATTR, "removexattr", false),
	byPath(unix.SYS_LREMOVEXATTR, "lremovexattr", true),
	byFd(unix.SYS_FREMOVEXATTR, "fremovexattr"),
}
