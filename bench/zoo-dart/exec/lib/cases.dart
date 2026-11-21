// The cells register their cases in register(); the harness calls them all. Every file of
// cells/lib is listed here (cmd/rivus TestZooTruthDart checks it).
import 'package:zoo_cells/call/direct.dart' as call_direct;

void registerAll() {
  call_direct.register();
}
