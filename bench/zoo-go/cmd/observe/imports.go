//go:build zooexec

package main

// The cells register themselves in init. Every directory under cells/ is listed here
// (cmd/rivus TestZooTruthGo checks it).
import (
	_ "zoo/cells/call/depth3"
	_ "zoo/cells/call/direct"
	_ "zoo/cells/call/direct_in"
	_ "zoo/cells/call/multiret"
	_ "zoo/cells/clos/inline"
	_ "zoo/cells/clos/returned"
	_ "zoo/cells/clos/to_own_helper"
	_ "zoo/cells/cont/map_key_vs_value"
	_ "zoo/cells/cont/slice"
	_ "zoo/cells/cont/struct_whole_to_sink"
	_ "zoo/cells/conv/buffer_writestring"
	_ "zoo/cells/dyn/http_middleware_chain"
	_ "zoo/cells/dyn/iface_ptr_recv_read"
	_ "zoo/cells/dyn/iface_val_recv"
	_ "zoo/cells/dyn/method_value"
	_ "zoo/cells/dyn/sink_in_method"
	_ "zoo/cells/dyn/stdlib_writer_own"
	_ "zoo/cells/dyn/xtalk_n_by_m"
	_ "zoo/cells/guard/one_branch"
	_ "zoo/cells/guard/sanitize_before"
	_ "zoo/cells/state/global_chain"
	_ "zoo/cells/state/global_direct"
	_ "zoo/cells/state/handler_then_reader"
	_ "zoo/cells/state/save_restore"
)
