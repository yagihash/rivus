//go:build linux && amd64

package sandbox

import "golang.org/x/sys/unix"

func seccompArch() uint32 { return unix.AUDIT_ARCH_X86_64 }

// seccompX32Bit: the x32 ABI has the numbers of x86-64 with this bit set, and the same audit architecture.
func seccompX32Bit() uint32 { return 0x40000000 }
