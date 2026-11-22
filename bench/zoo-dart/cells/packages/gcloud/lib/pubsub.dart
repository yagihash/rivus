// The skeleton of the Pub/Sub API of `package:gcloud` (see storage.dart): the name of a topic is the first argument.
import 'package:zoo_mark/zoo_mark.dart';

abstract interface class PubSub {
  factory PubSub(Object client, String project) => _SkeletonPubSub();

  Future<Topic> lookupTopic(String name);
}

abstract interface class Topic {
  String get name;
}

class _SkeletonPubSub implements PubSub {
  @override
  Future<Topic> lookupTopic(String name) async {
    recordAny(name);
    return _SkeletonTopic(name);
  }
}

class _SkeletonTopic implements Topic {
  _SkeletonTopic(this.name);

  @override
  final String name;
}
