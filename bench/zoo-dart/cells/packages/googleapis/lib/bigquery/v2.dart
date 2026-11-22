// The skeleton of the BigQuery API of `package:googleapis` (see ../../gcloud/lib/storage.dart for what a skeleton is):
// a request that holds the text of a query, and the resource that takes it. The real `QueryRequest` has more fields.
import 'package:zoo_mark/zoo_mark.dart';

class QueryRequest {
  QueryRequest({this.query, this.location});

  /// The text of the query: the dangerous field.
  String? query;

  /// The location of the job: a plain value.
  String? location;
}

class QueryResponse {
  QueryResponse({this.jobComplete});

  bool? jobComplete;
}

class JobsResource {
  Future<QueryResponse> query(QueryRequest request, String projectId) async {
    recordAny(request.query);
    return QueryResponse(jobComplete: true);
  }
}

class BigqueryApi {
  BigqueryApi(Object client);

  JobsResource get jobs => JobsResource();
}
