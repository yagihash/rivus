package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
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
	"rivus.ir/1": "161646c54971d05b4ec68eb1fc35cd605d4aca0f7a7d8ce8a3cb6c026d7f7fa7",
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
	enums, err := enumValues()
	if err != nil {
		panic(err)
	}
	lines = append(lines, enums...)
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

// stringEnums are the string types of ir.go whose constants are the values of the stream (the kinds of the instructions, the edges, ...): a
// value added to one is a change of the shape, and an extractor that is older never writes it. A string type of ir.go that is in neither list
// fails the test, so that a new enumeration cannot stay out of the hash.
var (
	stringEnums    = []string{"TypeClass", "EdgeKind", "TermKind", "Op", "ConstKind", "StepKind", "CalleeKind"}
	notEnumStrings = []string{"FuncID", "TypeID"}
)

// enumValues lists "Type=value" for the constants of the enumerations of ir.go, and fails for a string type that is not in the lists above.
func enumValues() ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "ir.go", nil, 0)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, n := range append(append([]string{}, stringEnums...), notEnumStrings...) {
		known[n] = true
	}
	isEnum := map[string]bool{}
	for _, n := range stringEnums {
		isEnum[n] = true
	}
	var out []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, sp := range g.Specs {
			switch sp := sp.(type) {
			case *ast.TypeSpec:
				if id, ok := sp.Type.(*ast.Ident); ok && id.Name == "string" && !known[sp.Name.Name] {
					return nil, fmt.Errorf("ir.go: the string type %s is in neither stringEnums nor notEnumStrings (schema_shape_test.go)", sp.Name.Name)
				}
			case *ast.ValueSpec:
				id, ok := sp.Type.(*ast.Ident)
				if g.Tok != token.CONST || !ok || !isEnum[id.Name] {
					continue
				}
				for i, v := range sp.Values {
					lit, ok := v.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return nil, fmt.Errorf("ir.go: %s is not a string literal", sp.Names[i].Name)
					}
					u, _ := strconv.Unquote(lit.Value)
					out = append(out, "enum "+id.Name+"="+u)
				}
			}
		}
	}
	// the type specs inside "type ( ... )" are visited too (Decls hold the group), so the list above is complete
	return out, nil
}

func TestEnumListCoversIR(t *testing.T) {
	vals, err := enumValues()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, v := range vals {
		seen[strings.SplitN(strings.TrimPrefix(v, "enum "), "=", 2)[0]] = true
	}
	for _, n := range stringEnums {
		if !seen[n] {
			t.Errorf("%s has no constants in ir.go", n)
		}
	}
}
