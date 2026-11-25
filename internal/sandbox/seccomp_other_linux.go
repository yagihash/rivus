//go:build linux && !amd64 && !arm64

package sandbox

// no filter is made for this architecture: Run says that the sandbox is not available.
func seccompArch() uint32 { return 0 }

func seccompX32Bit() uint32 { return 0 }
