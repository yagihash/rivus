import 'package:googleapis/calendar/v3.dart' as calendar;
import 'package:googleapis/cloudresourcemanager/v3.dart' as crm;
import 'package:googleapis/compute/v1.dart' as compute;
import 'package:googleapis/drive/v3.dart' as drive;
import 'package:googleapis/gmail/v1.dart' as gmail;
import 'package:googleapis/iam/v1.dart' as iam;
import 'package:googleapis/logging/v2.dart' as logging;
import 'package:googleapis/run/v2.dart' as run;
import 'package:googleapis/sheets/v4.dart' as sheets;
import 'package:googleapis/youtube/v3.dart' as youtube;
import 'package:googleapis_auth/auth_io.dart' as auth;
import 'package:grpc/grpc.dart';

import '../generated/cloud.pbgrpc.dart';

/// One call each to ten more APIs of the `googleapis` package. The ids come from the request.
class ManyApisService extends ManyApisServiceBase {
  @override
  Future<CloudReply> inspect(ServiceCall call, ObjectRequest request) async {
    final client = await auth.clientViaApplicationDefaultCredentials(
        scopes: ['https://www.googleapis.com/auth/cloud-platform']);
    final f = await drive.DriveApi(client).files.get(request.object);
    final v = await sheets.SheetsApi(client)
        .spreadsheets
        .values
        .get(request.bucket, request.key);
    final m = await gmail.GmailApi(client).users.messages.get('me', request.object);
    final i = await compute.ComputeApi(client)
        .instances
        .get('example-project', request.bucket, request.object);
    final y = await youtube.YouTubeApi(client)
        .videos
        .list(['snippet'], id: [request.object]);
    final e = await calendar.CalendarApi(client).events.list(request.bucket);
    final l = await logging.LoggingApi(client).entries.list(
        logging.ListLogEntriesRequest(filter: request.key));
    final s = await iam.IamApi(client).projects.serviceAccounts.get(request.object);
    final p = await crm.CloudResourceManagerApi(client).projects.get(request.object);
    final r = await run.CloudRunApi(client).projects.locations.services.get(request.object);
    return CloudReply()..body = '$f $v $m $i $y $e $l $s $p $r';
  }
}
