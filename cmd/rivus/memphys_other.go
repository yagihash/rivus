//go:build !darwin && !linux

package main

import "errors"

// physicalMemory: not known on this system.
func physicalMemory() (int64, error) {
	return 0, errors.New("the memory of the machine is not read on this system")
}
