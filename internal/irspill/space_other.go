//go:build !unix

package irspill

func freeSpace(string) int64 { return -1 }
