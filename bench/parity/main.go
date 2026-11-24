// Command parity makes the table of what the built-in model gives each language (B-52, stage 1): four numbers
// side by side, not one ratio. See README.md.
//
//	parity scan   -lang go|dart|rust -name N [-real] -ir FILE -out corpus-N.json
//	parity digest -lang L -name N -in result.json -out digest-N.json
//	parity report -work DIR [-table FILE] [-dead FILE] [-zoo zoo-lines.txt]
//
// The table and the list of the dead rows are not committed: the default is TABLE.md and DEAD.md in the -work directory.
//
// It reads the model through internal/model/builtin and the IR through its own small decoder; it does not touch the
// core.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: parity scan|digest|report ...")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "scan":
		err = cmdScan(os.Args[2:])
	case "digest":
		err = cmdDigest(os.Args[2:])
	case "report":
		err = cmdReport(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "parity:", err)
		os.Exit(1)
	}
}

func cmdScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	lang := fs.String("lang", "", "go, dart or rust")
	name := fs.String("name", "", "name of the corpus")
	real := fs.Bool("real", false, "the code of a real project (not a fixture)")
	in := fs.String("ir", "", "IR stream (JSON Lines)")
	out := fs.String("out", "", "output")
	note := fs.String("note", "", "a note for the table")
	supp := fs.Bool("supp", false, "shown beside the others, not in their sums")
	fs.Parse(args)
	c, err := scanFile(*in, *name, *lang, *real)
	if err != nil {
		return err
	}
	c.Note = *note
	c.Supp = *supp
	fmt.Fprintf(os.Stderr, "%s: %d functions, %d callee symbols\n", *name, c.Funcs, len(c.Calls))
	return c.save(*out)
}

func cmdDigest(args []string) error {
	fs := flag.NewFlagSet("digest", flag.ExitOnError)
	lang := fs.String("lang", "", "go, dart or rust")
	name := fs.String("name", "", "name of the corpus")
	in := fs.String("in", "", "the result of rivus analyze -pass-stats")
	out := fs.String("out", "", "output")
	note := fs.String("note", "", "a note for the table")
	fs.Parse(args)
	d, err := digestFile(*in, *name, *lang)
	if err != nil {
		return err
	}
	d.Note = *note
	return writeJSONFile(*out, d)
}

func cmdReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	work := fs.String("work", "", "directory with corpus-*.json and digest-*.json")
	table := fs.String("table", "", "the table (default: TABLE.md in the -work directory; not committed)")
	dead := fs.String("dead", "", "the list of the rows no IR hits (default: DEAD.md in the -work directory; not committed)")
	zoo := fs.String("zoo", "", "a file with the lines of the zoo tests (go test -v)")
	noGo := fs.Bool("no-gotypes", false, "do not load the types of the Go standard library")
	fs.Parse(args)

	if *table == "" {
		*table = filepath.Join(*work, "TABLE.md")
	}
	if *dead == "" {
		*dead = filepath.Join(*work, "DEAD.md")
	}
	rows, err := LoadRows()
	if err != nil {
		return err
	}
	mi := NewModelIndex(rows)
	files, _ := filepath.Glob(filepath.Join(*work, "corpus-*.json"))
	sort.Strings(files)
	var corpora []*Corpus
	for _, f := range files {
		c, err := loadCorpus(f)
		if err != nil {
			return err
		}
		corpora = append(corpora, c)
	}
	dfiles, _ := filepath.Glob(filepath.Join(*work, "digest-*.json"))
	sort.Strings(dfiles)
	digests := map[string]*Digest{}
	for _, f := range dfiles {
		d, err := loadDigest(f)
		if err != nil {
			return err
		}
		digests[d.Name] = d
	}
	var gu *GoUniverse
	if !*noGo {
		var extra []string
		seen := map[string]bool{}
		for _, r := range rows {
			if r.Lang == "go" && r.Call {
				if p := goPkgOf(r.Symbol); p != "" && strings.Contains(strings.SplitN(p, "/", 2)[0], ".") && !seen[p] {
					seen[p] = true
					extra = append(extra, p)
				}
			}
		}
		sort.Strings(extra)
		gu, err = LoadGoUniverse(".", extra)
		if err != nil {
			return fmt.Errorf("the types of Go: %w", err)
		}
	}
	var zooLines string
	if *zoo != "" {
		b, err := os.ReadFile(*zoo)
		if err == nil {
			zooLines = string(b)
		}
	}
	return render(*table, *dead, rows, mi, corpora, digests, gu, zooLines)
}

// goPkgOf is the import path of a go/types FullName.
func goPkgOf(sym string) string {
	if strings.HasPrefix(sym, "(") {
		i := strings.Index(sym, ")")
		if i < 0 {
			return ""
		}
		sym = strings.TrimPrefix(sym[1:i], "*")
	}
	sym = strings.SplitN(sym, "[", 2)[0]
	j := strings.LastIndex(sym, ".")
	if j < 0 {
		return ""
	}
	return sym[:j]
}
