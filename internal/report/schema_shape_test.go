package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// resultShapes is the hash of the shape of the result (the names and the types of the keys that the JSON carries, the values of the enumerations in
// resultEnums: kind, attributable_to, confidence and so on) for each version of the schema. The version goes up for every change of the shape, a
// key, a kind or a value that is added too: a reader that was written for an older version does not look for what is new, and nothing in the result
// says so (ADR-127, as ADR-125 for the IR). To change the shape: raise SchemaVersion in internal/report/report.go and add a row with the new
// hash to this table (the failure message prints the hash). Do not replace the hash of a version that was released.
//
// Not in the hash: the keys of the maps whose keys are free (the names are data, not a shape), and the wording of detail and of the other
// texts (the content, not the shape).
var resultShapes = map[string]string{
	"rivus.result/2": "615e46909f944ce9521908ce6a6a6bdcd1befbee027e28ca59ef759d23cdfc9c",
	"rivus.result/3": "cca52e17fe7a7fe820adabfeb4a99059f67f0553f940d16dec0e14c514c218da",
	"rivus.result/4": "9036c751e7b5aa8d9449e893f3927134a8340b5888728f2e61ad8c82e23d6fa2",
	"rivus.result/5": "b23fca971b5847edc3c837f563887d8b33a1a9771c9699f94c6b7295c071f069",
	"rivus.result/6": "7832a643ed1850ed5981b5a3f9d08493a8668f0da23665235010e51a15e4bb10",
}

// resultShape describes, for every type that the result reaches from a Report, the keys that are written to the JSON and their types, and the
// values of the enumerations of internal/taint/result.go. It is the same whatever the order of the declarations.
func resultShape() string {
	lines, err := shapeLines()
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

var marshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()

func shapeLines() ([]string, error) {
	seen := map[reflect.Type]bool{}
	var lines []string
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			t = t.Elem()
		}
		if t.Kind() == reflect.Map {
			// the type of the key is a part of the shape; the keys themselves are data
			walk(t.Key())
			walk(t.Elem())
			return
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		if t.Implements(marshalerType) || reflect.PointerTo(t).Implements(marshalerType) {
			// the JSON of this type is written by hand: its name is all that the shape can say
			lines = append(lines, t.String()+" {marshaler}")
			return
		}
		var fs []string
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() && !f.Anonymous {
				continue
			}
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
		sort.Strings(fs) // the order of the keys is not a shape
		lines = append(lines, t.String()+" {"+strings.Join(fs, "; ")+"}")
	}
	walk(reflect.TypeOf(Report{}))
	enums, err := enumValues()
	if err != nil {
		return nil, err
	}
	lines = append(lines, enums...)
	sort.Strings(lines)
	return lines, nil
}

// enumSource is a file and the constants of it that are values of the result: the ones whose name begins with Prefix. Label says whose values.
type enumSource struct {
	File, Prefix, Label string
}

// resultEnums are the enumerations of the result that the shape holds, with the constants that name their values. A value that is added is
// a change of the shape (a reader branches on the value; ADR-127). The enumerations that are not here are listed in ADR-127 (the addendum of
// the enumerations): a string whose values are not constants, or are texts.
var resultEnums = []enumSource{
	{"../taint/result.go", "Kind", "kind "},
	{"../taint/result.go", "Attributable", "attributable_to "},
	{"../taint/result.go", "Confidence", "confidence "},
	{"../taint/result.go", "Reach", "reachability "},
	{"../taint/result.go", "Cond", "conditional_node.kind "},
	{"report.go", "Reason", "summary.incomplete_reasons "},
	{"report.go", "Go", "summary.go_version.match "},
	{"report.go", "Dart", "summary.dart_version.match "},
	{"../../cmd/rivus/extractorcheck.go", "match", "summary.extractor.match "},
	{"../model/model.go", "Role", "model.role "},
	{"../sites/sites.go", "Role", "sites.role "},
	{"../model/model.go", "Origin", "model.origin "},
}

// enumValues lists "label value" for the constants of resultEnums: the kinds of the uncertainties (Kind...), the values of attributable_to
// (Attributable...), of confidence, of reachability, of the kind of a conditional node, of the reasons of incomplete, of
// summary.go_version.match and of summary.extractor.match. It fails for a constant of these names that is not a string literal.
func enumValues() ([]string, error) {
	var out []string
	for _, src := range resultEnums {
		f, err := parser.ParseFile(token.NewFileSet(), src.File, nil, 0)
		if err != nil {
			return nil, err
		}
		var n int
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, sp := range g.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if !strings.HasPrefix(name.Name, src.Prefix) {
						continue
					}
					if i >= len(vs.Values) {
						return nil, fmt.Errorf("%s: %s has no value", src.File, name.Name)
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return nil, fmt.Errorf("%s: %s is not a string literal", src.File, name.Name)
					}
					u, _ := strconv.Unquote(lit.Value)
					out = append(out, src.Label+u)
					n++
				}
			}
		}
		if n == 0 {
			return nil, fmt.Errorf("%s: no constant begins with %s", src.File, src.Prefix)
		}
	}
	return out, nil
}

func TestResultSchemaVersionMatchesShape(t *testing.T) {
	want, ok := resultShapes[SchemaVersion]
	got := resultShape()
	if !ok {
		t.Fatalf("SchemaVersion %q has no row in resultShapes. Add this row to resultShapes in internal/report/schema_shape_test.go:\n\t%q: %q,", SchemaVersion, SchemaVersion, got)
	}
	if got != want {
		next := SchemaVersion
		if i := strings.LastIndex(SchemaVersion, "/"); i >= 0 {
			if n, err := strconv.Atoi(SchemaVersion[i+1:]); err == nil {
				next = fmt.Sprintf("%s/%d", SchemaVersion[:i], n+1)
			}
		}
		t.Fatalf("the shape of the result changed (now %s, the row of %q has %s), and SchemaVersion is still %q. Change two places:\n"+
			"  1. internal/report/report.go: const SchemaVersion = %q\n"+
			"  2. internal/report/schema_shape_test.go: add this row to resultShapes (keep the row of %q):\n\t%q: %q,",
			got, SchemaVersion, want, SchemaVersion, next, SchemaVersion, next, got)
	}
}

func TestResultShapeCoversKinds(t *testing.T) {
	vals, err := enumValues()
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, v := range vals {
		count[strings.SplitN(v, " ", 2)[0]]++
	}
	// the numbers that the files have today: a constant that is added to one of these enumerations is a change of the shape, and the table above says so
	if count["kind"] < 20 || count["attributable_to"] != 3 || count["confidence"] != 2 || count["reachability"] != 3 || count["conditional_node.kind"] != 2 ||
		count["summary.incomplete_reasons"] != 5 || count["summary.go_version.match"] != 3 || count["summary.dart_version.match"] != 3 || count["summary.extractor.match"] != 6 ||
		count["model.role"] != 4 || count["sites.role"] != 2 || count["model.origin"] != 4 {
		t.Errorf("the constants of the enumerations were not read as expected: %v", count)
	}
}
