//go:build unix

package irspill

import "syscall"

// freeSpace is the bytes free in the directory for a user who is not root, or -1 when it is not known.
func freeSpace(dir string) int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return -1
	}
	return int64(st.Bavail) * int64(st.Bsize)
}
