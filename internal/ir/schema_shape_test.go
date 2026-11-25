package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// schemaShapes is the hash of the shape of the stream (the names and the types of the fields that the JSON carries) for each version of the
// schema. The version goes up for every change of the shape, an added field too: an extractor that is older than the reader does not write
// what the reader now expects, and nothing in the stream says so (#109). To change the shape: raise SchemaVersion here (internal/ir/ir.go),
// in extractor/rust/src/irjson.rs, in extractor/dart/lib/irjson.dart and in extractor/dart/bin/rivus_dart.dart, change the first line of the
// fixtures that hold a stream, and add the new hash to this table (the failure message prints it). Do not replace the hash of a version
// that was released.
var schemaShapes = map[string]string{
	"rivus.ir/1": "1b2ecc0963a4f95158fd1a488b81b077cfd700f5440d246c71cf0408e7b58753",
}

// shapeOf describes, for every type that the stream reaches from a record, the fields that are written to the JSON: the name of the key and the
// type. It is the same whatever the order of the declarations.
func shapeOf() string {
	seen := map[reflect.Type]bool{}
	var lines []string
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			t = t.Elem()
		}
		if t.Kind() == reflect.Map {
			walk(t.Key())
			walk(t.Elem())
			return
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		var fs []string
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag == "-" {
				continue
			}
			if tag == "" {
				tag = f.Name
			}
			fs = append(fs, fmt.Sprintf("%s %s", tag, f.Type.String()))
			walk(f.Type)
		}
		lines = append(lines, t.String()+" {"+strings.Join(fs, "; ")+"}")
	}
	walk(reflect.TypeOf(header{}))
	walk(reflect.TypeOf(record{}))
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func TestIRSchemaVersionMatchesShape(t *testing.T) {
	want, ok := schemaShapes[SchemaVersion]
	got := shapeOf()
	if !ok {
		t.Fatalf("SchemaVersion %q has no row in schemaShapes; its shape is %q", SchemaVersion, got)
	}
	if got != want {
		t.Fatalf("the shape of the stream changed (%s, want %s) and SchemaVersion is still %q: raise it (see schemaShapes), so that an older extractor is stopped and not read", got, want, SchemaVersion)
	}
}

// The extractors write their own copy of the version; one that was not raised with the reader would stop every run, or, if only the reader were
// raised, the stale extractor would be taken for a current one.
func TestExtractorsWriteTheSchemaVersion(t *testing.T) {
	for _, c := range []struct{ file, re string }{
		{"../../extractor/rust/src/irjson.rs", `SCHEMA_VERSION: &str = "([^"]+)"`},
		{"../../extractor/dart/lib/irjson.dart", `schemaVersion = '([^']+)'`},
		{"../../extractor/dart/bin/rivus_dart.dart", `rivus_dart (rivus\.ir/\d+)`},
	} {
		b, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(c.re).FindSubmatch(b)
		if m == nil || string(m[1]) != SchemaVersion {
			t.Errorf("%s does not write %q (found %q)", c.file, SchemaVersion, m)
		}
	}
}

func TestReadStopsAtAnOlderExtractor(t *testing.T) {
	in := `{"schema_version":"rivus.ir/0"}` + "\n"
	_, err := Read(strings.NewReader(in))
	if err == nil {
		t.Fatal("a stream of an older schema was read")
	}
	for _, w := range []string{"rivus.ir/0", SchemaVersion, "older"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("the message does not say %q: %v", w, err)
		}
	}
}
