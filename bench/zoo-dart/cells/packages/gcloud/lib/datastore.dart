// The skeleton of the Datastore API of `package:gcloud` (see storage.dart): the kind of a query is a named argument of
// the constructor of `Query`, which a row of the built-in model cannot point to.
import 'package:zoo_mark/zoo_mark.dart';

class Filter {
  Filter(this.relation, this.name, this.value);

  final String relation;
  final String name;
  final Object value;
}

class Query {
  Query({this.kind, this.filters});

  final String? kind;
  final List<Filter>? filters;
}

abstract interface class Datastore {
  factory Datastore(Object client, String project) => _SkeletonDatastore();

  Future<int> query(Query query);
}

class _SkeletonDatastore implements Datastore {
  @override
  Future<int> query(Query query) async {
    recordAny(query.kind);
    return 0;
  }
}
