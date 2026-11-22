import 'package:gcp_bench/src/services/apis_service.dart';
import 'package:gcp_bench/src/services/cloud_storage_service.dart';
import 'package:gcp_bench/src/services/files_service.dart';
import 'package:gcp_bench/src/services/gcloud_service.dart';
import 'package:gcp_bench/src/services/jobs_service.dart';
import 'package:gcp_bench/src/services/reports_service.dart';
import 'package:grpc/grpc.dart';

Future<void> main() async {
  final server = Server.create(services: [
    FilesService(),
    JobsService(),
    ReportsService(),
    GcloudService(),
    ApisService(),
    CloudStorageService(),
  ]);
  await server.serve(port: 50051);
  print('listening on ${server.port}');
}
