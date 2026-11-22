import 'package:grpc/grpc.dart';
import 'package:grpc_bench/src/services/files_service.dart';
import 'package:grpc_bench/src/services/jobs_service.dart';
import 'package:grpc_bench/src/services/reports_service.dart';

Future<void> main() async {
  final server = Server.create(services: [
    FilesService(),
    JobsService(),
    ReportsService(),
  ]);
  await server.serve(port: 50051);
  print('listening on ${server.port}');
}
