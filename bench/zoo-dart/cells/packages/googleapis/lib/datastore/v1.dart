// The skeleton of the Datastore API of `package:googleapis` (see ../../../gcloud/lib/storage.dart): a GQL query holds
// its text and its bindings (the values that the service binds). The skeleton records the text.
import 'package:zoo_mark/zoo_mark.dart';

class GqlQueryParameter {
  GqlQueryParameter({this.value});

  Object? value;
}

class GqlQuery {
  GqlQuery({this.queryString, this.namedBindings});

  String? queryString;
  Map<String, GqlQueryParameter>? namedBindings;
}

class RunQueryRequest {
  RunQueryRequest({this.gqlQuery});

  GqlQuery? gqlQuery;
}

class RunQueryResponse {}

class ProjectsResource {
  Future<RunQueryResponse> runQuery(RunQueryRequest request, String projectId) async {
    recordAny(request.gqlQuery?.queryString);
    return RunQueryResponse();
  }
}

class DatastoreApi {
  DatastoreApi(Object client);

  ProjectsResource get projects => ProjectsResource();
}
