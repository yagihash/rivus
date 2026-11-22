// The skeleton of `IOClient` of package:http: a class that has its own `send` (see client.dart).
import 'package:zoo_mark/zoo_mark.dart';

import 'request.dart';
import 'streamed_response.dart';

class IOClient {
  IOClient();

  Future<StreamedResponse> send(BaseRequest request) async {
    recordAny(Uri.decodeFull(request.url.toString()));
    return StreamedResponse(200);
  }
}
