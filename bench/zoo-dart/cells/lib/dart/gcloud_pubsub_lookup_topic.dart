// @cell(F/dart/gcloud-pubsub-lookup-topic)
// @shape: a value is the name of the topic that PubSub.lookupTopic of package:gcloud (a skeleton of it) looks up: the resource_name sink of the built-in rows
import 'package:gcloud/pubsub.dart' as pubsub;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/gcloud-pubsub-lookup-topic/vuln', vuln);
  zooCase('F/dart/gcloud-pubsub-lookup-topic/fixed', fixed);
  zooCase('F/dart/gcloud-pubsub-lookup-topic/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final ps = pubsub.PubSub(Object(), 'p');
  final v = src('dart-gcloud-pubsub-lookup-topic-vuln'); // @source(dart-gcloud-pubsub-lookup-topic-vuln)
  await ps.lookupTopic(v); // @sink(dart-gcloud-pubsub-lookup-topic-vuln)
}

Future<void> fixed() async { // @fixed
  final ps = pubsub.PubSub(Object(), 'p');
  final v = cleanPath(cleanHtml(cleanSql(cleanAll(src('dart-gcloud-pubsub-lookup-topic-fixed')))));
  await ps.lookupTopic(v); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final ps = pubsub.PubSub(Object(), 'p');
  final v = src('dart-gcloud-pubsub-lookup-topic-decoy'); // @source(dart-gcloud-pubsub-lookup-topic-decoy)
  final names = <String, String>{v: 'events'};
  await ps.lookupTopic(names[v] ?? 'events'); // @sink(none) @nosink(dart-gcloud-pubsub-lookup-topic-decoy)
  sinkAny(v); // @sink(dart-gcloud-pubsub-lookup-topic-decoy)
}
