// Command rivus is a taint analysis query engine: it reports facts about
// where untrusted data can reach, and why it could not tell in some places.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

const usage = `rivus - taint analysis query engine

Usage:
  rivus <command> [flags] [packages]

Commands:
  sites     list the places where source and sink rules match (fast)
  analyze   find the flows from sources to sinks across functions and packages
  query     answer a one-sided question about a sink or a source
  index     read the code once and save its call graph (and with -taint, the analysis) for later queries
  graph     search the call graph saved by index ("rivus graph info"): calls, not the flow of data
  resolve   find where the places seen earlier (bookmarks) are in the current code
  extract   write the neutral IR of the code (JSON Lines, or -text to read it)
  models    "rivus models coverage": which functions need a model most
  version   print the version (also --version)

Run "rivus <command> -h" for the flags of a command.
`

func main() {
	tuneGC()
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "sites":
		err = runSites(os.Args[2:])
	case "analyze":
		err = runAnalyze(os.Args[2:])
	case "query":
		err = runQuery(os.Args[2:])
	case "index":
		err = runIndex(os.Args[2:])
	case "graph":
		err = runGraph(os.Args[2:])
	case "resolve":
		err = runResolve(os.Args[2:])
	case "extract":
		err = runExtract(os.Args[2:])
	case "models":
		err = runModels(os.Args[2:])
	case "version", "-version", "--version":
		fmt.Println(versionString())
		return
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "rivus: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "rivus: %v\n", err)
		os.Exit(1)
	}
}

// tuneGC makes the collector keep less garbage than its default. The analysis
// holds a few GB of live data on a large program, and the default (let the heap
// double before a collection) doubled the peak; half costs almost no time.
// GOGC set in the environment wins.
func tuneGC() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(50)
	}
}
