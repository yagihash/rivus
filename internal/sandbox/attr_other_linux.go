//go:build linux && !amd64 && !arm64

package sandbox

// attrCalls and attrDenied: there is no filter for this architecture (see seccomp_other_linux.go).
var (
	attrCalls  []attrCall
	attrDenied []uint32
)
