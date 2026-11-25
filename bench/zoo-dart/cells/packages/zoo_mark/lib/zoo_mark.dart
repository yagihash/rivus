// What the cases of the zoo call: the sources, the sinks and the sanitizers. This is the file that the
// analysis reads: the declarations only, with empty bodies, so that no state of the recording is seen by
// the analysis (every case would write to one global). rivus gets the meaning of each function from
// rules.yaml. The recording is in bench/zoo-dart/exec/zoo_mark (the harness puts it in place of this
// package with dependency_overrides). The two have the same declarations, and the test of the zoo holds
// them to it.
import 'dart:async';

/// The source that is a parameter type (rules.yaml: param_type). The handler under test takes an [In]; a
/// case makes one with `In('tag')`.
class In {
  In(String tag) : value = tag;
  final String value;
}

/// The structures that the sinks of the cells read a part of (docs/DESIGN-field-sinks.md): the shapes of the
/// requests of a generated API.
///
/// Stmt is a statement whose text is dangerous and whose bind parameters are not.
class Stmt {
  Stmt({this.sql = '', Map<String, String>? params}) : params = params ?? <String, String>{};
  String sql;
  Map<String, String> params;
}

/// CreateTask is the request of a service that makes a task, and Task the task: an object that holds an object
/// that holds a sealed class (a oneof) whose value is a wrapper that holds the request of the HTTP call.
class CreateTask {
  CreateTask({this.parent = '', this.task});
  String parent;
  Task? task;
}

/// Task has the oneof message.
class Task {
  Task({this.name = '', this.message});
  String name;
  TaskMessage? message;
}

sealed class TaskMessage {}

/// The wrapper of the HTTP request in the oneof.
class TaskHttpRequest extends TaskMessage {
  TaskHttpRequest({this.httpRequest});
  HttpRequest? httpRequest;
}

/// The request: the url is the sink (the body is not).
class HttpRequest {
  HttpRequest({this.url = '', this.body = ''});
  String url;
  String body;
}

/// The marker of the tag (a source).
String src(String tag) => '';

/// What the user gives to the framework in a case (the text typed, the arguments of a route, the message of a
/// channel): the marker of the tag at run time, and **not a source** of rules.yaml: the source of such a cell is
/// the API of the framework (a field that is read, the parameter of a method that an app overrides).
String typed(String tag) => tag;

/// The sinks of each class. They record what they got and execute nothing.
void sinkCmd(String s) {}

/// sinkCmd for a place that takes a function with a result of bool (the function of Iterable.where).
bool sinkCmdB(String s) => false;

void sinkCmdBytes(List<int> b) {}
void sinkSql(String s) {}
void sinkHtml(String s) {}
void sinkPath(String base, String s) {}

/// Runs the statement: the sql of it is the sink, the params are not (rules.yaml names the field).
void sinkStmt(Stmt s) {}

/// sinkStmt for a statement that is held as an Object.
void sinkStmtAny(Object v) {}

/// Sends the request of the task: the url of the HTTP request is the sink.
void sinkTask(CreateTask r) {}

/// A function that the analysis has no body and no model of (the declaration has no body). The recording
/// one moves the "body" param of a statement to its sql, which is what a function that is not read may do.
external void opaque(Stmt s);

/// A sink for structs and containers only: the text of the value (its `toString`) is what is recorded.
void sinkAny(Object? v) {}

/// What a skeleton of a library records of what it was given (packages/http): the same as [sinkAny] at run time, and
/// **not a sink of rules.yaml**: the sink of such a cell is the built-in row of the library, at the call of the cell.
void recordAny(Object? v) {}

/// The sanitizers: each replaces the dangerous characters of its class by '_'.
String cleanCmd(String s) => '';
// the kinds that the zoo has no judgement for: a URL that a request goes to (ssrf, open_redirect), the name of a resource
String cleanUrl(String s) => '';
String cleanResource(String s) => '';
String cleanSql(String s) => '';
String cleanHtml(String s) => '';
String cleanPath(String s) => '';

/// Registers a case of the zoo (the harness runs it).
void zooCase(String id, FutureOr<void> Function() fn) {}
