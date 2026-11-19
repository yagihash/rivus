import 'package:grpc/grpc.dart';

import 'lib.dart';

// A gRPC service as protoc_plugin writes it: the handlers are abstract methods
// of a subclass of Service, each taking a ServiceCall and the request, and the
// code of the project overrides them. What the client sent is the input.

class EchoRequest {
  String name = '';
}

abstract class EchoServiceBase extends Service {
  @override
  String get $name => 'Echo';

  // what the generated code wraps around a handler: not an entry point of its own
  Future<String> echo_Pre(ServiceCall call, Future<EchoRequest> request) async {
    return echo(call, await request);
  }

  Future<String> echo(ServiceCall call, EchoRequest request);
  Future<String> many(ServiceCall call, Stream<EchoRequest> request);
}

class EchoService extends EchoServiceBase {
  @override // @source(grpc-unary)
  Future<String> echo(ServiceCall call, EchoRequest request) async {
    sink(request.name); // @sink(grpc-unary)
    return 'ok';
  }

  @override // @source(grpc-stream)
  Future<String> many(ServiceCall call, Stream<EchoRequest> request) async {
    await for (final r in request) {
      sink(r.name); // @sink(grpc-stream)
    }
    return 'ok';
  }

  // not a handler: it does not take a ServiceCall
  void helper(EchoRequest request) {
    sink(request.name); // @sink(none)
  }
}

void serve() {
  Server.create(services: [EchoService()]);
}
