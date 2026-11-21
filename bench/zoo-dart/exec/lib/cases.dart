// The cells register their cases in register(); the harness calls them all. Every file of
// cells/lib is listed here (cmd/rivus TestZooTruthDart checks it).
import 'package:zoo_cells/call/depth3.dart' as call_depth3;
import 'package:zoo_cells/call/direct.dart' as call_direct;
import 'package:zoo_cells/clos/two_callers.dart' as clos_two_callers;
import 'package:zoo_cells/cont/list.dart' as cont_list;
import 'package:zoo_cells/cont/map.dart' as cont_map;
import 'package:zoo_cells/dart/async_await.dart' as dart_async_await;
import 'package:zoo_cells/dart/callable_class.dart' as dart_callable_class;
import 'package:zoo_cells/dart/extension.dart' as dart_extension;
import 'package:zoo_cells/dart/for_in.dart' as dart_for_in;
import 'package:zoo_cells/dart/future_then.dart' as dart_future_then;
import 'package:zoo_cells/dart/getter_setter.dart' as dart_getter_setter;
import 'package:zoo_cells/dart/interp_tostring.dart' as dart_interp_tostring;
import 'package:zoo_cells/dart/named_args.dart' as dart_named_args;
import 'package:zoo_cells/dart/patterns_record.dart' as dart_patterns_record;
import 'package:zoo_cells/dart/super_mixin_impl.dart' as dart_super_mixin_impl;
import 'package:zoo_cells/dart/tearoff_instance.dart' as dart_tearoff_instance;
import 'package:zoo_cells/guard/one_branch.dart' as guard_one_branch;
import 'package:zoo_cells/guard/sanitize_before.dart' as guard_sanitize_before;
import 'package:zoo_cells/state/global_direct.dart' as state_global_direct;
import 'package:zoo_cells/state/handler_then_reader.dart' as state_handler_then_reader;
import 'package:zoo_cells/state/provider_store.dart' as state_provider_store;

void registerAll() {
  call_depth3.register();
  call_direct.register();
  clos_two_callers.register();
  cont_list.register();
  cont_map.register();
  dart_async_await.register();
  dart_callable_class.register();
  dart_extension.register();
  dart_for_in.register();
  dart_future_then.register();
  dart_getter_setter.register();
  dart_interp_tostring.register();
  dart_named_args.register();
  dart_patterns_record.register();
  dart_super_mixin_impl.register();
  dart_tearoff_instance.register();
  guard_one_branch.register();
  guard_sanitize_before.register();
  state_global_direct.register();
  state_handler_then_reader.register();
  state_provider_store.register();
}
