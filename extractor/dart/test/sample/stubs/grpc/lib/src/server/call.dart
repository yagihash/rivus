// The part of package:grpc that the sample needs, with the names and the
// places of the real one (the rules name them).
abstract class ServiceCall {
  Map<String, String>? get clientMetadata;
}
