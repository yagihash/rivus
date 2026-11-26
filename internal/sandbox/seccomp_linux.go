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
	bpfJgt   = 0x25 // BPF_JMP | BPF_JGT | BPF_K
	bpfAnd   = 0x54 // BPF_ALU | BPF_AND | BPF_K
	bpfRet   = 0x06 // BPF_RET | BPF_K

	offNr   = 0  // seccomp_data.nr
	offArch = 4  // seccomp_data.arch
	offArg0 = 16 // seccomp_data.args[0] (the low word: little endian, which both architectures are)
	offArg1 = 24
	offArg2 = 32
	offArg3 = 40

	afInet     = 2
	afInet6    = 10
	sockStream = 1
	ipprotoTCP = 6
)

// The highest number of a system call that the lists below were made with (the highest SYS_* of golang.org/x/sys/unix of the architecture, at the time they were
// reviewed). The filter is a list of what is refused, so a call that is newer than the list (io_uring was one) would be let through by default: a number above
// the ceiling is told that the call is not there (ENOSYS), as clone3 is. When golang.org/x/sys moves the highest number up, TestSandboxSeccompCeilingCoversXSys fails:
// the new calls are to be read, put in deniedCalls or in the other lists when they need it, and the ceiling is raised.
const (
	seccompMaxNrAmd64 = 471 // SYS_RSEQ_SLICE_YIELD
	seccompMaxNrArm64 = 471 // SYS_RSEQ_SLICE_YIELD
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
	// the IPC that the system shares among the processes of a user and that Landlock does not mediate (the IPC namespace is the host's, and a namespace cannot be made:
	// see below): SysV shared memory, message queues and semaphores (a segment of another process of the same user, such as the buffers of an X11 client, can be
	// attached, read and written), POSIX message queues, and kcmp (what two processes share). A build needs none of them.
	unix.SYS_SHMGET, unix.SYS_SHMAT, unix.SYS_SHMCTL, unix.SYS_SHMDT,
	unix.SYS_MSGGET, unix.SYS_MSGSND, unix.SYS_MSGRCV, unix.SYS_MSGCTL,
	unix.SYS_SEMGET, unix.SYS_SEMOP, unix.SYS_SEMTIMEDOP, unix.SYS_SEMCTL,
	unix.SYS_MQ_OPEN, unix.SYS_MQ_UNLINK, unix.SYS_MQ_TIMEDSEND, unix.SYS_MQ_TIMEDRECEIVE, unix.SYS_MQ_NOTIFY, unix.SYS_MQ_GETSETATTR,
	unix.SYS_KCMP,
	// the memory of another process, by its pidfd (they need rights that a user has over its own processes)
	unix.SYS_PROCESS_MADVISE, unix.SYS_PROCESS_MRELEASE,
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

// argEq is a condition on an argument of a call (the low 32 bits: the kernel takes these arguments as int).
type argEq struct{ off, val uint32 }

// ownOnly are the calls that change something of a PROCESS (a limit, the priority, the cpus, the scheduler, the memory policy) and that take the process as an argument:
// a user may do them to every process of its own, and Landlock mediates only the signals. The program may do them to itself (0 is the caller), and to nobody else: with
// another number of a process, or "all the processes of the user" (PRIO_USER, IOPRIO_WHO_USER), the answer is EPERM. (A build that runs with a limit of its own, nice or
// taskset, does it to itself.)
var ownOnly = []struct {
	nr   uint32
	args []argEq
}{
	{unix.SYS_PRLIMIT64, []argEq{{offArg0, 0}}},
	{unix.SYS_SETPRIORITY, []argEq{{offArg0, 0 /* PRIO_PROCESS */}, {offArg1, 0}}},
	{unix.SYS_IOPRIO_SET, []argEq{{offArg0, 1 /* IOPRIO_WHO_PROCESS */}, {offArg1, 0}}},
	{unix.SYS_SCHED_SETAFFINITY, []argEq{{offArg0, 0}}},
	{unix.SYS_SCHED_SETSCHEDULER, []argEq{{offArg0, 0}}},
	{unix.SYS_SCHED_SETPARAM, []argEq{{offArg0, 0}}},
	{unix.SYS_SCHED_SETATTR, []argEq{{offArg0, 0}}},
	{unix.SYS_MIGRATE_PAGES, []argEq{{offArg0, 0}}},
	{unix.SYS_MOVE_PAGES, []argEq{{offArg0, 0}}},
}

// ownOnlyRule is the block of the filter of one of those calls: when the call is another one, the block is skipped; when every argument is the value, the call goes on;
// otherwise it is refused. (The accumulator holds the number of the call when the block begins and when it ends.)
func ownOnlyRule(nr uint32, eqs []argEq) []unix.SockFilter {
	k := len(eqs)
	n := 2*k + 3 // the test of the call, k pairs (load, compare), the refusal, the load of the number again
	deny := 2*k + 1
	out := []unix.SockFilter{ins(bpfJeq, 0, uint8(n-1), nr)}
	for i, e := range eqs {
		out = append(out, ins(bpfLdAbs, 0, 0, e.off))
		at := 2*i + 2
		if i == k-1 {
			out = append(out, ins(bpfJeq, 1, 0, e.val)) // the last one: when it is the value, the refusal is jumped over
		} else {
			out = append(out, ins(bpfJeq, 0, uint8(deny-(at+1)), e.val)) // when it is not the value: to the refusal
		}
	}
	return append(out, ins(bpfRet, 0, 0, retEPERM), ins(bpfLdAbs, 0, 0, offNr))
}

// maskRule is the block of the filter of one call whose argument (masked) is compared with a value: when the call is another one, the block is skipped;
// with allowEqual the call goes on only when it is equal and is refused otherwise, and without it the call is refused when it is equal.
// (The accumulator holds the number of the call when the block ends.)
func maskRule(nr uint32, off, mask, val uint32, allowEqual bool) []unix.SockFilter {
	jt, jf := uint8(0), uint8(1)
	if allowEqual {
		jt, jf = 1, 0
	}
	return []unix.SockFilter{
		ins(bpfJeq, 0, 4, nr),
		ins(bpfLdAbs, 0, 0, off),
		ins(bpfAnd, 0, 0, mask),
		ins(bpfJeq, jt, jf, val),
		ins(bpfRet, 0, 0, retEPERM),
		ins(bpfLdAbs, 0, 0, offNr),
	}
}

// the commands of ioctl that set the flags of an inode (chattr, the project id, nodump, noatime ...): they work on a descriptor opened for reading, Landlock does
// not mediate the ioctls of a regular file, and they are attribute changes that the supervisor does not see (like file_setattr, which is refused).
var inodeFlagIoctls = []uint32{0x40086602 /* FS_IOC_SETFLAGS */, 0x40046602 /* FS_IOC32_SETFLAGS */, 0x401c5820 /* FS_IOC_FSSETXATTR */}

const msgFastopen = 0x20000000

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
	// a number above the ceiling: the call is not there (the x32 numbers are above it too, and were killed before)
	p = append(p, ins(bpfJgt, 0, 1, seccompMaxNr()), ins(bpfRet, 0, 0, unix.SECCOMP_RET_ERRNO|uint32(unix.ENOSYS)))
	for _, nr := range deniedCalls {
		p = append(p, ins(bpfJeq, 0, 1, nr), deny)
	}
	if notify {
		for _, c := range attrCalls {
			p = append(p, ins(bpfJeq, 0, 1, c.nr), ins(bpfRet, 0, 0, unix.SECCOMP_RET_USER_NOTIF))
		}
		for _, nr := range attrDenied {
			p = append(p, ins(bpfJeq, 0, 1, nr), deny)
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
	for _, r := range ownOnly {
		p = append(p, ownOnlyRule(r.nr, r.args)...)
	}
	for _, c := range inodeFlagIoctls {
		p = append(p, maskRule(unix.SYS_IOCTL, offArg1, 0xffffffff, c, false)...)
	}
	if net != netOpen {
		// socketpair makes two ends that are joined and no path, when it is a stream or a seqpacket: a datagram end can send to, and bind, any address
		// mask 0xb: SOCK_STREAM (1) and SOCK_SEQPACKET (5), which Rust's std uses; both are connected ends that cannot be sent elsewhere
		p = append(p, maskRule(unix.SYS_SOCKETPAIR, offArg1, 0xb, sockStream, true)...)
	}
	switch net {
	case netNone:
		// no socket of any family: no TCP, no UDP, no raw, no UNIX socket (a UNIX socket of a path is the way to the agents of the user: ssh-agent,
		// the Docker socket). socketpair stays, for streams only (above).
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
		// TCP Fast Open: sendto, sendmsg and sendmmsg with MSG_FASTOPEN connect on their own, without the call that the rule of the port of Landlock looks at
		p = append(p, maskRule(unix.SYS_SENDTO, offArg3, msgFastopen, 0, true)...)
		p = append(p, maskRule(unix.SYS_SENDMSG, offArg2, msgFastopen, 0, true)...)
		p = append(p, maskRule(unix.SYS_SENDMMSG, offArg3, msgFastopen, 0, true)...)
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
