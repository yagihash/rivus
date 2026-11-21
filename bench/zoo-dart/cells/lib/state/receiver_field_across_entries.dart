// @cell(F/state/receiver-field-across-entries)
// @shape: one entry point (an event handler) writes a field of the receiver, and another entry point (build, dispose) reads it; the closure of setState does the writing
import 'package:zoo_flutter/zoo_flutter.dart';
import 'package:zoo_mark/zoo_mark.dart';

void register() {
  zooCase('F/state/receiver-field-across-entries/vuln', vuln);
  zooCase('F/state/receiver-field-across-entries/vuln2', vuln2);
  zooCase('F/state/receiver-field-across-entries/fixed', fixed);
  zooCase('F/state/receiver-field-across-entries/decoy', decoy);
}

class SearchPage extends State {
  String _query = '';

  void onChanged(String v) {
    setState(() {
      _query = v;
    });
  }

  @override
  void build() {
    sinkCmd(_query); // @sink(state-receiver-field-across-entries-vuln)
  }
}

void vuln() { // @vuln
  final page = SearchPage();
  page.onChanged(src('state-receiver-field-across-entries-vuln')); // @source(state-receiver-field-across-entries-vuln)
  page.build();
}

// initState puts a value in the field, dispose reads it
class TokenPage extends State {
  String _token = '';

  @override
  void initState() {
    _token = src('state-receiver-field-across-entries-vuln2'); // @source(state-receiver-field-across-entries-vuln2)
  }

  @override
  void build() {}

  @override
  void dispose() {
    sinkCmd(_token); // @sink(state-receiver-field-across-entries-vuln2)
  }
}

void vuln2() { // @vuln
  final page = TokenPage();
  page.initState();
  page.build();
  page.dispose();
}

class SearchPageFixed extends State {
  String _query = '';

  void onChanged(String v) {
    setState(() {
      _query = cleanCmd(v);
    });
  }

  @override
  void build() {
    sinkCmd(_query); // @sink(none)
  }
}

void fixed() { // @fixed
  final page = SearchPageFixed();
  page.onChanged(src('state-receiver-field-across-entries-fixed'));
  page.build();
}

class SearchPageDecoy extends State {
  String _query = '';
  String _title = '';

  void onChanged(String v) {
    setState(() {
      _query = v;
      _title = 'constant';
    });
  }

  @override
  void build() {
    sinkCmd(_title); // @sink(none) @nosink(state-receiver-field-across-entries-decoy)
    sinkSql(_query); // @sink(state-receiver-field-across-entries-decoy)
  }
}

void decoy() { // @decoy
  final page = SearchPageDecoy();
  page.onChanged(src('state-receiver-field-across-entries-decoy')); // @source(state-receiver-field-across-entries-decoy)
  page.build();
}
