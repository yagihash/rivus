package main

import (
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

// The memory limit of the run (ADR-121). It is the soft limit of the Go runtime (debug.SetMemoryLimit, the same as GOMEMLIMIT):
// when the memory of the process nears it, the collector works harder. It changes how the memory is used, not the result.

const (
	memLimitFlag    = "flag"
	memLimitEnv     = "env"
	memLimitDefault = "default"
	memLimitNone    = "none"

	gib = int64(1) << 30
)

// memoryLimit is what applyMemoryLimit decided, and what the summary says (memory_limit_*).
type memoryLimit struct {
	// Bytes is the limit; 0 is none.
	Bytes  int64
	Source string
	Note   string
	// set is whether debug.SetMemoryLimit has to be called (the limit of the environment is applied by the runtime already).
	set bool
}

// parseMemorySize reads a size: a number of bytes, or a number with a unit (KiB MiB GiB TiB: powers of 1024; KB MB GB TB:
// powers of 1000; case does not matter). "off" is no limit (off = true).
func parseMemorySize(s string) (n int64, off bool, err error) {
	bad := fmt.Errorf("invalid size %q: write a number of bytes, or a number with a unit (512MiB, 6GiB, 6GB; KiB MiB GiB TiB, KB MB GB TB), or off", s)
	t := strings.ToLower(strings.TrimSpace(s))
	if t == "off" {
		return 0, true, nil
	}
	units := []struct {
		suffix string
		mult   int64
	}{
		{"kib", 1 << 10}, {"mib", 1 << 20}, {"gib", 1 << 30}, {"tib", 1 << 40},
		{"kb", 1000}, {"mb", 1000 * 1000}, {"gb", 1000 * 1000 * 1000}, {"tb", 1000 * 1000 * 1000 * 1000},
		{"b", 1},
	}
	mult := int64(1)
	for _, u := range units {
		if strings.HasSuffix(t, u.suffix) {
			t, mult = strings.TrimSpace(strings.TrimSuffix(t, u.suffix)), u.mult
			break
		}
	}
	v, perr := strconv.ParseInt(t, 10, 64)
	if perr != nil || v <= 0 || v > math.MaxInt64/mult {
		return 0, false, bad
	}
	return v * mult, false, nil
}

// defaultMemoryLimit is the limit when none is given: a quarter of the memory of the machine (of the cgroup, when it is smaller),
// and on a small machine at least min(4 GiB, half of it). physical and cgroup are bytes; 0 is not known (cgroup 0: no limit).
// ok is false when the memory of the machine is not known.
func defaultMemoryLimit(physical, cgroup int64) (limit int64, ok bool) {
	if physical <= 0 {
		return 0, false
	}
	base := physical
	if cgroup > 0 && cgroup < base {
		base = cgroup
	}
	return max(base/4, min(4*gib, base/2)), true
}

// decideMemoryLimit picks the limit: -max-memory, then GOMEMLIMIT, then the default. flag is the value of -max-memory ("" when it was
// not given); envValue is GOMEMLIMIT; envLimit is the limit the runtime has set from it (math.MaxInt64: none); physical and cgroup as
// in defaultMemoryLimit, physErr why the memory of the machine is not known.
func decideMemoryLimit(flag, envValue string, envLimit, physical, cgroup int64, physErr string) (memoryLimit, error) {
	if flag != "" {
		n, off, err := parseMemorySize(flag)
		if err != nil {
			return memoryLimit{}, err
		}
		m := memoryLimit{Source: memLimitFlag, set: true}
		if off {
			m.Source, m.Bytes = memLimitNone, 0
			m.Note = "-max-memory off"
		} else {
			m.Bytes = n
		}
		if envValue != "" {
			m.Note = joinNote(m.Note, fmt.Sprintf("-max-memory overrides GOMEMLIMIT=%s", envValue))
		}
		return m, nil
	}
	if envValue != "" {
		m := memoryLimit{Source: memLimitEnv}
		if envLimit < math.MaxInt64 {
			m.Bytes = envLimit
		} else {
			m.Note = "GOMEMLIMIT=" + envValue + " sets no limit"
		}
		return m, nil
	}
	n, ok := defaultMemoryLimit(physical, cgroup)
	if !ok {
		if physErr == "" {
			physErr = "the memory of the machine is not known"
		}
		return memoryLimit{Source: memLimitNone, Note: "no default limit: " + physErr}, nil
	}
	return memoryLimit{Bytes: n, Source: memLimitDefault, set: true}, nil
}

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

// applyMemoryLimit decides the limit from the flag, the environment and the machine, and sets it. Called before the analysis reads anything.
func applyMemoryLimit(flag string) (memoryLimit, error) {
	envValue := os.Getenv("GOMEMLIMIT")
	var physical, cgroup int64
	var physErr string
	if flag == "" && envValue == "" {
		var err error
		if physical, err = physicalMemory(); err != nil {
			physErr = err.Error()
		}
		cgroup = cgroupMemoryLimit()
	}
	m, err := decideMemoryLimit(flag, envValue, debug.SetMemoryLimit(-1), physical, cgroup, physErr)
	if err != nil {
		return m, err
	}
	if m.set {
		if m.Bytes > 0 {
			debug.SetMemoryLimit(m.Bytes)
			if os.Getenv("GOGC") == "" {
				debug.SetGCPercent(gogcFor(m.Bytes))
			}
		} else {
			debug.SetMemoryLimit(math.MaxInt64)
		}
	}
	return m, nil
}

// cgroupMemoryLimit is the memory limit of the cgroup of this process (Linux: v2, then v1); 0 when there is none or it is not known.
func cgroupMemoryLimit() int64 {
	for _, p := range []string{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory/memory.limit_in_bytes"} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(b))
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 || n >= 1<<60 { // "max" (v2) and the huge number of v1 are no limit
			return 0
		}
		return n
	}
	return 0
}
