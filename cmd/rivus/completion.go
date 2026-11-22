package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// "rivus completion <shell>" writes the script that completes rivus in a shell. The script is made from the table of commands (commands.go) and from the
// flags that the commands define, on an empty flag set (nothing is run, and no extractor or network is needed), so a command or a flag that is added
// is completed without anything else being written. A shell is one line of completionShells: its name, its summary, and the function that writes its
// script from the table.

// completionShells are the shells that rivus writes a completion script for. They are the commands under "rivus completion".
var completionShells = []*command{
	{name: "zsh", summary: "the completion script for zsh: source <(rivus completion zsh), or save it as _rivus in a directory of $fpath", run: runCompletionZsh},
}

func runCompletionZsh(args []string) error {
	return runCompletion("completion zsh", args, writeZshCompletion)
}

// runCompletion parses the arguments of "rivus completion <shell>" (it has no flags besides -h) and writes the script with write.
func runCompletion(name string, args []string, write func(w io.Writer, cmds []*command) error) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	prepareFlags(fs, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%s takes no arguments (got %q)", name, fs.Arg(0))
	}
	return write(os.Stdout, commands)
}

// completionFlag is a flag as the completion needs it.
type completionFlag struct {
	name, usage, argName string
	// takesValue is false for a flag that is a bool (it stands alone).
	takesValue bool
	diagnostic bool
	// kind is what the value is: "dir", "file", "enum:<name>", or "".
	kind string
}

// completionFlags lists the flags of a command: the flags that it defines, on an empty flag set, with the side and the kind of value that the registry of
// flagsides.go has for each, and the flags that every command with flags has (-h, -help-all).
func completionFlags(c *command) []completionFlag {
	if c.flags == nil {
		return nil
	}
	fs := flag.NewFlagSet(c.name, flag.ContinueOnError)
	c.flags(fs)
	s := sidesOf(fs)
	var out []completionFlag
	fs.VisitAll(func(f *flag.Flag) {
		argName, usage := flag.UnquoteUsage(f)
		bf, isBool := f.Value.(interface{ IsBoolFlag() bool })
		out = append(out, completionFlag{
			name: f.Name, usage: firstSentence(usage), argName: argName,
			takesValue: !(isBool && bf.IsBoolFlag()),
			diagnostic: s.diag[f.Name],
			kind:       s.kinds[f.Name],
		})
	})
	out = append(out, completionFlag{name: "h", usage: "show the flags of the command"}, completionFlag{name: helpAllFlag, usage: helpAllUsage})
	return out
}

// maxDescription is the longest description that the menu of a shell gets: the flags have long usages, of which the first sentence is the description.
const maxDescription = 110

// firstSentence is the first sentence of a usage, on one line, and cut when it is long.
func firstSentence(usage string) string {
	usage = strings.Join(strings.Fields(usage), " ")
	if i := strings.Index(usage, ". "); i >= 0 {
		usage = usage[:i]
	}
	usage = strings.TrimSuffix(usage, ".")
	if len(usage) > maxDescription {
		cut := strings.LastIndex(usage[:maxDescription], " ")
		if cut < 0 {
			cut = maxDescription
		}
		usage = usage[:cut] + "..."
	}
	return usage
}

// zshQuote quotes s as one word of zsh with single quotes.
func zshQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// zshSpecText escapes text that goes inside an option spec of _arguments: [explanation], or the message after a colon.
func zshSpecText(s string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`, `:`, `\:`).Replace(s)
}

// zshDescribe makes the entries of _describe ("name:description"): the colon of the name is escaped, the description goes as it is.
func zshDescribe(name, desc string) string {
	return strings.ReplaceAll(name, ":", `\:`) + ":" + desc
}

// zshFunc is the name of the function that completes the command at path ("_rivus_graph_callers").
func zshFunc(path []string) string {
	return "_" + strings.Join(append([]string{"rivus"}, path...), "_")
}

// zshAction is the action of _arguments for the value of a flag.
func zshAction(kind string) string {
	switch {
	case kind == "dir":
		return "_files -/"
	case kind == "file":
		return "_files"
	case strings.HasPrefix(kind, "enum:"):
		return "(" + strings.Join(enumFlags[strings.TrimPrefix(kind, "enum:")].values, " ") + ")"
	}
	return ""
}

// diagnosticMark starts the description of a flag for diagnosis and tuning.
const diagnosticMark = "(diagnostic) "

// writeZshCompletion writes the zsh script for the commands of cs.
func writeZshCompletion(w io.Writer, cs []*command) error {
	var b strings.Builder
	b.WriteString("#compdef rivus\n\n")
	b.WriteString("# Completion of rivus for zsh. It is made by \"rivus completion zsh\" from the commands and the flags of this build of rivus.\n")
	b.WriteString("# Use it with: source <(rivus completion zsh)   (after compinit), or save it: rivus completion zsh > \"${fpath[1]}/_rivus\"\n\n")
	zshCommandList(&b, nil, &command{name: "rivus", sub: cs})
	b.WriteString("\ncompdef _rivus rivus\n")
	// when the file is the function (loaded from $fpath), the first call runs it
	b.WriteString("\nif [[ $funcstack[1] == _rivus ]]; then\n  _rivus \"$@\"\nfi\n")
	_, err := io.WriteString(w, b.String())
	return err
}

// zshCommandList writes the functions of the command c (which has commands under it) and of everything under it.
func zshCommandList(b *strings.Builder, path []string, c *command) {
	var usual, other []*command
	for _, s := range c.sub {
		if s.usual {
			usual = append(usual, s)
		} else {
			other = append(other, s)
		}
	}
	fn := zshFunc(path)
	fmt.Fprintf(b, "%s() {\n", fn)
	b.WriteString("  local curcontext=\"$curcontext\" state state_descr line ret=1\n  typeset -A opt_args\n  local -a usual other\n")
	entries := func(name string, cs []*command) {
		fmt.Fprintf(b, "  %s=(\n", name)
		for _, s := range cs {
			fmt.Fprintf(b, "    %s\n", zshQuote(zshDescribe(s.name, s.summary)))
		}
		b.WriteString("  )\n")
	}
	entries("usual", usual)
	entries("other", other)
	b.WriteString("  _arguments -C '1: :->command' '*:: :->args' && ret=0\n")
	b.WriteString("  case $state in\n    command)\n")
	if len(usual) > 0 {
		b.WriteString("      _describe -t commands 'command' usual && ret=0\n")
	}
	if len(other) > 0 {
		b.WriteString("      _describe -t other-commands 'other command' other && ret=0\n")
	}
	b.WriteString("      ;;\n    args)\n      case $line[1] in\n")
	for _, s := range c.sub {
		fmt.Fprintf(b, "        %s) %s && ret=0 ;;\n", s.name, zshFunc(append(slices.Clone(path), s.name)))
	}
	b.WriteString("      esac\n      ;;\n  esac\n  return ret\n}\n")
	for _, s := range c.sub {
		p := append(slices.Clone(path), s.name)
		b.WriteString("\n")
		if len(s.sub) > 0 {
			zshCommandList(b, p, s)
		} else {
			zshLeaf(b, p, s)
		}
	}
}

// zshLeaf writes the function of a command that has flags and no commands under it.
func zshLeaf(b *strings.Builder, path []string, c *command) {
	fmt.Fprintf(b, "%s() {\n  _arguments \\\n", zshFunc(path))
	flags := completionFlags(c)
	// the usual flags first, then the ones for diagnosis (which say so)
	slices.SortStableFunc(flags, func(x, y completionFlag) int {
		if x.diagnostic == y.diagnostic {
			return 0
		}
		if x.diagnostic {
			return 1
		}
		return -1
	})
	for _, f := range flags {
		desc := zshSpecText(f.usage)
		if f.diagnostic {
			desc = diagnosticMark + desc
		}
		spec := "-" + f.name
		if f.takesValue {
			action := zshAction(f.kind)
			msg := f.argName
			if f.kind == "dir" || f.kind == "file" {
				msg = f.kind
			} else if strings.HasPrefix(f.kind, "enum:") {
				msg = f.name
			}
			spec += "=[" + desc + "]:" + zshSpecText(msg) + ":" + action
		} else {
			spec += "[" + desc + "]"
		}
		fmt.Fprintf(b, "    %s \\\n", zshQuote(spec))
	}
	if c.args != "" {
		b.WriteString("    '*:file or directory:_files'\n")
	} else {
		b.WriteString("    '*: :_message \"no more arguments\"'\n")
	}
	b.WriteString("}\n")
}
