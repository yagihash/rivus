//go:build linux && !amd64 && !arm64

package sandbox

import "fmt"

// There is no filter of seccomp for this architecture (the filter is made for x86-64 and arm64: the numbers of the system calls, the audit architecture), and
// the sandbox does not run without one: the helper says that it cannot be made, and nothing is run. The package builds, so that the rest of rivus does.
func seccompArch() uint32 { return 0 }

func seccomp(cfg *helperConfig) (int, error) {
	return -1, fmt.Errorf("seccomp: this architecture is not one that the sandbox knows (x86-64 and arm64 are)")
}
