//go:build linux && arm64

package sandbox

import "golang.org/x/sys/unix"

// attrCalls: see attr_amd64_linux.go (arm64 has only the forms that take a descriptor or a directory).
var attrCalls = []attrCall{
	{unix.SYS_FCHMOD, "fchmod", 0, -1, -1},
	{unix.SYS_FCHMODAT, "fchmodat", -1, 0, 1},
	{unix.SYS_FCHMODAT2, "fchmodat2", -1, 0, 1},
	{unix.SYS_FCHOWN, "fchown", 0, -1, -1},
	{unix.SYS_FCHOWNAT, "fchownat", -1, 0, 1},
	{unix.SYS_UTIMENSAT, "utimensat", -1, 0, 1},
	{unix.SYS_SETXATTR, "setxattr", -1, -1, 0},
	{unix.SYS_LSETXATTR, "lsetxattr", -1, -1, 0},
	{unix.SYS_FSETXATTR, "fsetxattr", 0, -1, -1},
	{unix.SYS_REMOVEXATTR, "removexattr", -1, -1, 0},
	{unix.SYS_LREMOVEXATTR, "lremovexattr", -1, -1, 0},
	{unix.SYS_FREMOVEXATTR, "fremovexattr", 0, -1, -1},
}
