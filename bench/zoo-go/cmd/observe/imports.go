//go:build zooexec

package main

// The cells register themselves in init. Every directory under cells/ is listed here
// (cmd/rivus TestZooTruthGo checks it).
import (
	_ "zoo/cells/call/depth3"
	_ "zoo/cells/call/direct"
	_ "zoo/cells/call/direct_in"
)
