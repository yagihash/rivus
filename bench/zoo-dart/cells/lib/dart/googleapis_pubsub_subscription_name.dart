// @cell(F/dart/googleapis-pubsub-subscription-name)
// Go side: not measured yet (no Go cell of this form in zoo-go-pkg or bench/gcp); not a form that Go lacks. Counted as a Go todo by TestZooDart (no mark)
// @shape: a value is the name of the subscription of ProjectsSubscriptionsResource.modifyPushConfig of package:googleapis (a skeleton of it), a call whose row of the built-in model is the push endpoint only (one category for one symbol)
import 'package:googleapis/pubsub/v1.dart' as pubsub;
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/dart/googleapis-pubsub-subscription-name/vuln', vuln);
  zooCase('F/dart/googleapis-pubsub-subscription-name/fixed', fixed);
  zooCase('F/dart/googleapis-pubsub-subscription-name/decoy', decoy);
}

Future<void> vuln() async { // @vuln
  final api = pubsub.PubsubApi(Object());
  final v = src('dart-googleapis-pubsub-subscription-name-vuln'); // @source(dart-googleapis-pubsub-subscription-name-vuln) @miss(unmodeled_library_call) @ref(#140)
  await api.projects.subscriptions.modifyPushConfig(pubsub.ModifyPushConfigRequest(pushConfig: pubsub.PushConfig(pushEndpoint: 'https://example.invalid/')), v); // @sink(dart-googleapis-pubsub-subscription-name-vuln)
}

Future<void> fixed() async { // @fixed
  final api = pubsub.PubsubApi(Object());
  final v = cleanPath(cleanHtml(cleanSql(cleanCmd(src('dart-googleapis-pubsub-subscription-name-fixed')))));
  await api.projects.subscriptions.modifyPushConfig(pubsub.ModifyPushConfigRequest(pushConfig: pubsub.PushConfig(pushEndpoint: 'https://example.invalid/')), v); // @sink(none)
}

Future<void> decoy() async { // @decoy
  final api = pubsub.PubsubApi(Object());
  final v = src('dart-googleapis-pubsub-subscription-name-decoy'); // @source(dart-googleapis-pubsub-subscription-name-decoy)
  final names = <String, String>{v: 'events-sub'};
  await api.projects.subscriptions.modifyPushConfig(pubsub.ModifyPushConfigRequest(pushConfig: pubsub.PushConfig(pushEndpoint: 'https://example.invalid/')), names[v] ?? 'events-sub'); // @sink(none) @nosink(dart-googleapis-pubsub-subscription-name-decoy)
  sinkAny(v); // @sink(dart-googleapis-pubsub-subscription-name-decoy)
}
