//go:build linux && (amd64 || arm64)

package sandbox

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// seccomp closes what Landlock does not reach: the sockets of every kind (Landlock knows TCP only), and the system calls that go round it
// or round the user's other processes. The filter is a list: what is named is refused with EPERM, the rest is allowed (the files are
// Landlock's). It is made for the architecture it runs on (seccompArch): a program that makes a system call by another ABI of the same machine
// (the x32 numbers on x86-64) is killed, since the numbers below would not be its.

// BPF instructions (linux/bpf_common.h, linux/filter.h)
const (
	bpfLdAbs = 0x20 // BPF_LD | BPF_W | BPF_ABS
	bpfJeq   = 0x15 // BPF_JMP | BPF_JEQ | BPF_K
	bpfJge   = 0x35 // BPF_JMP | BPF_JGE | BPF_K
	bpfAnd   = 0x54 // BPF_ALU | BPF_AND | BPF_K
	bpfRet   = 0x06 // BPF_RET | BPF_K

	offNr   = 0  // seccomp_data.nr
	offArch = 4  // seccomp_data.arch
	offArg0 = 16 // seccomp_data.args[0] (the low word: little endian, which both architectures are)
	offArg1 = 24
	offArg2 = 32

	afInet     = 2
	afInet6    = 10
	sockStream = 1
	ipprotoTCP = 6
)

var retEPERM = uint32(unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM))

func ins(code uint16, jt, jf uint8, k uint32) unix.SockFilter {
	return unix.SockFilter{Code: code, Jt: jt, Jf: jf, K: k}
}

// deniedCalls are refused whatever the network is.
var deniedCalls = []uint32{
	unix.SYS_PTRACE,            // another process of the user, and the memory of a process
	unix.SYS_PROCESS_VM_READV,  // the memory of another process of the user
	unix.SYS_PROCESS_VM_WRITEV, // (the same, written)
	unix.SYS_PIDFD_GETFD,       // a file descriptor of another process of the user
	unix.SYS_IO_URING_SETUP,    // io_uring makes sockets and opens files without the system calls that are filtered here
	unix.SYS_IO_URING_ENTER,
	unix.SYS_IO_URING_REGISTER,
	unix.SYS_BPF,
	unix.SYS_PERF_EVENT_OPEN,
	unix.SYS_USERFAULTFD,
	unix.SYS_KEYCTL, // the keys of the user's session
	unix.SYS_ADD_KEY,
	unix.SYS_REQUEST_KEY,
	// a new user namespace gives a process the rights of root inside it, and with them a lot of the kernel that an unprivileged process cannot reach
	// (mounts, networks, firewalls): a way into kernel bugs. Nothing that a build runs needs one.
	unix.SYS_UNSHARE,
	unix.SYS_SETNS,
	// what only root can do (the program has no capability: see dropCapabilities; these stay refused if that was not enough)
	unix.SYS_MOUNT, unix.SYS_UMOUNT2, unix.SYS_PIVOT_ROOT, unix.SYS_CHROOT,
	unix.SYS_FSOPEN, unix.SYS_FSCONFIG, unix.SYS_FSMOUNT, unix.SYS_MOVE_MOUNT, unix.SYS_MOUNT_SETATTR,
	unix.SYS_INIT_MODULE, unix.SYS_FINIT_MODULE, unix.SYS_DELETE_MODULE, unix.SYS_KEXEC_LOAD, unix.SYS_KEXEC_FILE_LOAD,
	unix.SYS_REBOOT, unix.SYS_SWAPON, unix.SYS_SWAPOFF,
}

// cloneNewFlags are the flags of clone that make a new namespace (CLONE_NEWNS, NEWCGROUP, NEWUTS, NEWIPC, NEWUSER, NEWPID, NEWNET, NEWTIME).
const cloneNewFlags = 0x7e020080

// filter makes the program for the network of the run. With notify, the calls that change the attributes of a file (attrCalls) are handed to the
// process that started the run, which looks at them and lets them through (notify.go).
func filter(net int, notify bool) ([]unix.SockFilter, error) {
	arch := seccompArch()
	if arch == 0 {
		return nil, fmt.Errorf("seccomp: this architecture is not one that the sandbox knows (x86-64 and arm64 are)")
	}
	kill := ins(bpfRet, 0, 0, unix.SECCOMP_RET_KILL_PROCESS)
	deny := ins(bpfRet, 0, 0, retEPERM)
	allow := ins(bpfRet, 0, 0, unix.SECCOMP_RET_ALLOW)
	p := []unix.SockFilter{
		ins(bpfLdAbs, 0, 0, offArch),
		ins(bpfJeq, 1, 0, arch), // this architecture: go on
		kill,
		ins(bpfLdAbs, 0, 0, offNr),
	}
	if x32 := seccompX32Bit(); x32 != 0 {
		p = append(p, ins(bpfJge, 0, 1, x32), kill)
	}
	for _, nr := range deniedCalls {
		p = append(p, ins(bpfJeq, 0, 1, nr), deny)
	}
	if notify {
		for _, c := range attrCalls {
			p = append(p, ins(bpfJeq, 0, 1, c.nr), ins(bpfRet, 0, 0, unix.SECCOMP_RET_USER_NOTIF))
		}
	}
	// clone3 is told that it is not there (the C library goes back to clone, which has its flags in a register that can be looked at: clone3
	// has them in a structure in memory, which a filter cannot read); clone with a flag that makes a namespace is refused
	p = append(p,
		ins(bpfJeq, 0, 1, unix.SYS_CLONE3), ins(bpfRet, 0, 0, unix.SECCOMP_RET_ERRNO|uint32(unix.ENOSYS)),
		ins(bpfJeq, 0, 4, unix.SYS_CLONE),
		ins(bpfLdAbs, 0, 0, offArg0),
		ins(bpfAnd, 0, 0, cloneNewFlags),
		ins(bpfJeq, 1, 0, 0),
		deny,
		ins(bpfLdAbs, 0, 0, offNr), // (the accumulator held the flags)
	)
	switch net {
	case netNone:
		// no socket of any family: no TCP, no UDP, no raw, no UNIX socket (a UNIX socket of a path is the way to the agents of the user: ssh-agent,
		// the Docker socket). socketpair stays: it makes two ends that are joined, and no path.
		p = append(p, ins(bpfJeq, 0, 1, unix.SYS_SOCKET), deny)
	case netProxy:
		// only a TCP socket of IPv4 or IPv6 (Landlock then lets it connect to the one port): no UDP, no raw, no UNIX socket. And the protocol is the default
		// one (0) or TCP: a stream socket of another protocol (SCTP, MPTCP) is not covered by the rule of the port, which knows TCP only.
		p = append(p,
			ins(bpfJeq, 0, 12, unix.SYS_SOCKET),
			ins(bpfLdAbs, 0, 0, offArg0),
			ins(bpfJeq, 2, 0, afInet), // -> load the type
			ins(bpfJeq, 1, 0, afInet6),
			deny,
			ins(bpfLdAbs, 0, 0, offArg1),
			ins(bpfAnd, 0, 0, 0xf), // the type, without SOCK_NONBLOCK and SOCK_CLOEXEC
			ins(bpfJeq, 1, 0, sockStream),
			deny,
			ins(bpfLdAbs, 0, 0, offArg2),
			ins(bpfJeq, 2, 0, 0),
			ins(bpfJeq, 1, 0, ipprotoTCP),
			deny,
		)
		// (the test of the call jumps over the 12 instructions that follow it when the call is another one)
	case netOpen:
		// the fetch stage: the sockets are open
	}
	p = append(p, allow)
	return p, nil
}

// seccomp installs the filter. With cfg.Notify it returns the file descriptor of the notifications (-1 otherwise).
func seccomp(cfg *helperConfig) (int, error) {
	f, err := filter(cfg.Net, cfg.Notify)
	if err != nil {
		return -1, err
	}
	prog := unix.SockFprog{Len: uint16(len(f)), Filter: &f[0]}
	flags := uintptr(0)
	if cfg.Notify {
		flags = unix.SECCOMP_FILTER_FLAG_NEW_LISTENER
	}
	r, _, errno := unix.Syscall(unix.SYS_SECCOMP, unix.SECCOMP_SET_MODE_FILTER, flags, uintptr(unsafe.Pointer(&prog)))
	if errno != 0 {
		return -1, fmt.Errorf("seccomp: %v", errno)
	}
	if cfg.Notify {
		return int(r), nil
	}
	return -1, nil
}
