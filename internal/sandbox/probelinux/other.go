//go:build linux

package main

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

var (
	victim     = flag.Int("victim", 0, "the number of a process of the same user that is not this one: its limit and its priority are changed (the sandbox says EPERM)")
	selfLimits = flag.Bool("self-limits", false, "the limit and the priority of this process itself are changed (they are allowed)")
)

func othersExtra() {
	if *victim > 0 {
		say("victim-prlimit", unix.Prlimit(*victim, unix.RLIMIT_NOFILE, &unix.Rlimit{Cur: 3, Max: 3}, nil))
		say("victim-setpriority", unix.Setpriority(unix.PRIO_PROCESS, *victim, 19))
		say("victim-setpriority-user", unix.Setpriority(unix.PRIO_USER, 0x7ffffff0, 19)) // (a user that nobody is: nothing is changed even outside of the sandbox, where it is ESRCH; inside it is refused for the kind of the target)
	}
	if *selfLimits {
		var cur unix.Rlimit
		unix.Getrlimit(unix.RLIMIT_NOFILE, &cur)
		say("self-setrlimit", unix.Setrlimit(unix.RLIMIT_NOFILE, &cur))
		say("self-prlimit-pid", unix.Prlimit(os.Getpid(), unix.RLIMIT_NOFILE, &cur, nil))
		say("self-setpriority", unix.Setpriority(unix.PRIO_PROCESS, 0, 5))
		fmt.Println("self-done")
	}
}
