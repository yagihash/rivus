typedef Handler = void Function(String s);

abstract class Listener {
  void onEvent(String name);
}

class Destination {
  String bucketName = '';
  int count = 0;
}

class Document {
  static const String contentTypeHeader = 'content-type';
  String body = '';
  String title = '';
  final List<int> raw = const [];
  String _private = '';
}

class Client {
  Client(String host, {int port = 0});
  Client.fromUrl(Uri url);

  Future<String> get(String url, {String? savePath, Object? data}) async => '';
  void run(String command) {}
  void open(String path, int mode) {}
  void sql(dynamic query) {}
  Future<String> content() async => '';
  Map<String, String> get headers => {};
  void move(Destination d) {}
  void listen(Listener l, Handler h) {}
  void plain(String s) {}
  void bucket(String name) {}
  set baseUrl(String v) {}
  void _hidden(String query) {}
}

Future<Document> fetchDocument(String url) async => Document();
