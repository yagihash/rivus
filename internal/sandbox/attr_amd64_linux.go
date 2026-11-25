//go:build linux && amd64

package sandbox

import "golang.org/x/sys/unix"

// attrCalls are the system calls that change the mode, the owner, the times or the extended attributes of a file, and that rivus does for the program (notify_linux.go):
// Landlock does not stop them. The forms that a build and the tools that it runs use are here.
var attrCalls = []attrCall{
	chmodPath(unix.SYS_CHMOD, "chmod"),
	chmodFd(unix.SYS_FCHMOD, "fchmod"),
	chmodAt(unix.SYS_FCHMODAT, "fchmodat", -1),
	chmodAt(unix.SYS_FCHMODAT2, "fchmodat2", 3),
	chownPath(unix.SYS_CHOWN, "chown", false),
	chownPath(unix.SYS_LCHOWN, "lchown", true),
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

// attrDenied are the other forms (the old ones that the C library does not use any more, and the newest ones): they change attributes and are not done for the program
// and are refused (EPERM): a form that is done for the program has to be understood to the last argument, and these are not needed by what a build runs.
var attrDenied = []uint32{
	unix.SYS_UTIME, unix.SYS_UTIMES, unix.SYS_FUTIMESAT,
	unix.SYS_SETXATTRAT, unix.SYS_REMOVEXATTRAT, unix.SYS_FILE_SETATTR,
}
