//go:build linux && arm64

package sandbox

import "golang.org/x/sys/unix"

func seccompArch() uint32 { return unix.AUDIT_ARCH_AARCH64 }

func seccompX32Bit() uint32 { return 0 }

// seccompMaxNr: the numbers above it are told that the call is not there (seccompMaxNrArm64).
func seccompMaxNr() uint32 { return seccompMaxNrArm64 }
