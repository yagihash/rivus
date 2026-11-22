/// A request (the shape of `package:http`: the method and the URL).
abstract class BaseRequest {
  BaseRequest(this.method, this.url);
  final String method;
  final Uri url;
}

class Request extends BaseRequest {
  Request(super.method, super.url);
}
