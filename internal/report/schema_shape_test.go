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

// resultShapes is the hash of the shape of the result (the names and the types of the keys that the JSON carries, the values of the kinds of
// the uncertainties and the values of attributable_to) for each version of the schema. The version goes up for every change of the shape, a
// key or a kind that is added too: a reader that was written for an older version does not look for what is new, and nothing in the result
// says so (ADR-127, as ADR-125 for the IR). To change the shape: raise SchemaVersion in internal/report/report.go and add a row with the new
// hash to this table (the failure message prints the hash). Do not replace the hash of a version that was released.
//
// Not in the hash: the keys of the maps whose keys are free (the names are data, not a shape), and the wording of detail and of the other
// texts (the content, not the shape).
var resultShapes = map[string]string{
	"rivus.result/2": "615e46909f944ce9521908ce6a6a6bdcd1befbee027e28ca59ef759d23cdfc9c",
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

// enumValues lists "name=value" for the constants of internal/taint/result.go that are values of the result: the kinds of the uncertainties
// (Kind...) and the values of attributable_to (Attributable...). It fails for a constant of these names that is not a string literal.
func enumValues() ([]string, error) {
	const file = "../taint/result.go"
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, sp := range g.Specs {
			vs := sp.(*ast.ValueSpec)
			for i, n := range vs.Names {
				var prefix string
				switch {
				case strings.HasPrefix(n.Name, "Kind"):
					prefix = "kind "
				case strings.HasPrefix(n.Name, "Attributable"):
					prefix = "attributable_to "
				default:
					continue
				}
				if i >= len(vs.Values) {
					return nil, fmt.Errorf("%s: %s has no value", file, n.Name)
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return nil, fmt.Errorf("%s: %s is not a string literal", file, n.Name)
				}
				u, _ := strconv.Unquote(lit.Value)
				out = append(out, prefix+u)
			}
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
	var kinds, attrs int
	for _, v := range vals {
		if strings.HasPrefix(v, "kind ") {
			kinds++
		} else {
			attrs++
		}
	}
	if kinds < 20 || attrs != 3 {
		t.Errorf("the constants of result.go were not read as expected: %d kinds, %d attributable_to values", kinds, attrs)
	}
}
