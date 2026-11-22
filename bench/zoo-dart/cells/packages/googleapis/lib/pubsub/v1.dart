// The skeleton of the Pub/Sub API of `package:googleapis` (see ../../../gcloud/lib/storage.dart for what a skeleton is):
// the name of a subscription (the second argument) and the push endpoint of a request. The skeleton records both.
import 'package:zoo_mark/zoo_mark.dart';

class PushConfig {
  PushConfig({this.pushEndpoint});

  String? pushEndpoint;
}

class ModifyPushConfigRequest {
  ModifyPushConfigRequest({this.pushConfig});

  PushConfig? pushConfig;
}

class Empty {}

class ProjectsSubscriptionsResource {
  Future<Empty> modifyPushConfig(ModifyPushConfigRequest request, String subscription) async {
    recordAny(subscription);
    recordAny(request.pushConfig?.pushEndpoint);
    return Empty();
  }
}

class ProjectsResource {
  ProjectsSubscriptionsResource get subscriptions => ProjectsSubscriptionsResource();
}

class PubsubApi {
  PubsubApi(Object client);

  ProjectsResource get projects => ProjectsResource();
}
