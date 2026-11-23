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

class ViewDefinition {
  ViewDefinition({this.query, this.useLegacySql});

  /// A query that BigQuery executes when the view is referenced: the dangerous field.
  String? query;

  bool? useLegacySql;
}

class MaterializedViewDefinition {
  MaterializedViewDefinition({this.query, this.enableRefresh});

  /// A query whose result is persisted: the dangerous field.
  String? query;

  bool? enableRefresh;
}

class Table {
  Table({this.view, this.materializedView, this.description, this.friendlyName});

  ViewDefinition? view;
  MaterializedViewDefinition? materializedView;

  /// Plain text: not SQL.
  String? description;

  String? friendlyName;
}

class TablesResource {
  Future<Table> insert(Table request, String projectId, String datasetId) async {
    recordAny(request.view?.query);
    recordAny(request.materializedView?.query);
    return request;
  }

  Future<Table> patch(Table request, String projectId, String datasetId, String tableId) async {
    recordAny(request.view?.query);
    recordAny(request.materializedView?.query);
    return request;
  }

  Future<Table> update(Table request, String projectId, String datasetId, String tableId) async {
    recordAny(request.view?.query);
    recordAny(request.materializedView?.query);
    return request;
  }
}

class Routine {
  Routine({this.definitionBody, this.description});

  /// The body of the routine: the dangerous field.
  String? definitionBody;

  /// Plain text: not SQL.
  String? description;
}

class RoutinesResource {
  Future<Routine> insert(Routine request, String projectId, String datasetId) async {
    recordAny(request.definitionBody);
    return request;
  }

  Future<Routine> update(Routine request, String projectId, String datasetId, String routineId) async {
    recordAny(request.definitionBody);
    return request;
  }
}

class BigqueryApi {
  BigqueryApi(Object client);

  JobsResource get jobs => JobsResource();

  TablesResource get tables => TablesResource();

  RoutinesResource get routines => RoutinesResource();
}
