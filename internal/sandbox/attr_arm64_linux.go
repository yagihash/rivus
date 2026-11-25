//go:build linux && arm64

package sandbox

import "golang.org/x/sys/unix"

// attrCalls: see attr_amd64_linux.go (arm64 has only the forms that take a descriptor or a directory, and the ones of the extended attributes).
var attrCalls = []attrCall{
	chmodFd(unix.SYS_FCHMOD, "fchmod"),
	chmodAt(unix.SYS_FCHMODAT, "fchmodat", -1),
	chmodAt(unix.SYS_FCHMODAT2, "fchmodat2", 3),
	chownFd(unix.SYS_FCHOWN, "fchown"),
	chownAt(unix.SYS_FCHOWNAT, "fchownat"),
	utimensat(unix.SYS_UTIMENSAT),
	setxattrPath(unix.SYS_SETXATTR, "setxattr", false),
	setxattrPath(unix.SYS_LSETXATTR, "lsetxattr", true),
	setxattrFd(unix.SYS_FSETXATTR, "fsetxattr"),
	removexattrPath(unix.SYS_REMOVEXATTR, "removexattr", false),
	removexattrPath(unix.SYS_LREMOVEXATTR, "lremovexattr", true),
	removexattrFd(unix.SYS_FREMOVEXATTR, "fremovexattr"),
}

// attrDenied: see attr_amd64_linux.go.
var attrDenied = []uint32{unix.SYS_SETXATTRAT, unix.SYS_REMOVEXATTRAT, unix.SYS_FILE_SETATTR}
