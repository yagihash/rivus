// @cell(F/conv/struct-field-deep)
// @shape: the request of a generated API (an object that holds an object that holds a sealed class (a oneof) whose value is a wrapper) is given to the call, and the rule of the sink names the url, four fields down (docs/DESIGN-field-sinks.md)
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/conv/struct-field-deep/vuln', vuln);
  zooCase('F/conv/struct-field-deep/fixed', fixed);
  zooCase('F/conv/struct-field-deep/decoy', decoy);
}

CreateTask build(HttpRequest h) {
  return CreateTask(task: Task(message: TaskHttpRequest(httpRequest: h)));
}

void vuln() { // @vuln
  final h = HttpRequest();
  h.url = src('conv-struct-field-deep-vuln'); // @source(conv-struct-field-deep-vuln)
  sinkTask(build(h)); // @sink(conv-struct-field-deep-vuln)
}

void fixed() { // @fixed
  final h = HttpRequest();
  h.url = cleanCmd(src('conv-struct-field-deep-fixed'));
  sinkTask(build(h)); // @sink(none)
}

void decoy() { // @decoy
  final t = src('conv-struct-field-deep-decoy'); // @source(conv-struct-field-deep-decoy)
  final h = HttpRequest(url: 'https://example.com/');
  h.body = t;
  sinkTask(build(h)); // @sink(none) @fp @nosink(conv-struct-field-deep-decoy) @fp @ref(FIELD-SINK)
  sinkCmd(t); // @sink(conv-struct-field-deep-decoy)
}
