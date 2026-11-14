package gossa

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/ssa"
)

const maxSnippetLines = 12

// Snippet returns the source lines of the statement enclosing pos. For
// compound statements (if, for, switch, ...) only the header is returned.
func (p *Program) Snippet(pos token.Pos) string {
	start, end, ok := p.stmtRange(pos)
	if !ok {
		return ""
	}
	return p.lines(start, end)
}

func (p *Program) stmtRange(pos token.Pos) (token.Pos, token.Pos, bool) {
	f := p.File(pos)
	if f == nil {
		return 0, 0, false
	}
	path, _ := astutil.PathEnclosingInterval(f, pos, pos)
	for _, n := range path {
		switch s := n.(type) {
		case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
			continue
		case *ast.IfStmt:
			return s.Pos(), s.Body.Lbrace, true
		case *ast.ForStmt:
			return s.Pos(), s.Body.Lbrace, true
		case *ast.RangeStmt:
			return s.Pos(), s.Body.Lbrace, true
		case *ast.SwitchStmt:
			return s.Pos(), s.Body.Lbrace, true
		case *ast.TypeSwitchStmt:
			return s.Pos(), s.Body.Lbrace, true
		case *ast.SelectStmt:
			return s.Pos(), s.Body.Lbrace, true
		case ast.Stmt:
			return s.Pos(), s.End(), true
		case *ast.ValueSpec:
			return s.Pos(), s.End(), true
		case *ast.FuncDecl:
			if s.Body != nil {
				return s.Pos(), s.Body.Lbrace, true
			}
			return s.Pos(), s.End(), true
		}
	}
	return 0, 0, false
}

// lines returns the whole source lines covering [start, end).
func (p *Program) lines(start, end token.Pos) string {
	tf := p.Fset.File(start)
	if tf == nil {
		return ""
	}
	src := p.source(tf.Name())
	if src == nil {
		return ""
	}
	first := tf.Line(start)
	last := tf.Line(end)
	if end > start && last > first {
		// end is exclusive; if it sits at column 1 the previous line is the last.
		if tf.Offset(end) == tf.Offset(tf.LineStart(last)) {
			last--
		}
	}
	if last < first {
		last = first
	}
	var out []string
	for l := first; l <= last && l-first < maxSnippetLines; l++ {
		so := tf.Offset(tf.LineStart(l))
		eo := len(src)
		if l < tf.LineCount() {
			eo = tf.Offset(tf.LineStart(l + 1))
		}
		out = append(out, strings.TrimRight(string(src[so:eo]), "\r\n"))
	}
	return dedent(out)
}

func dedent(lines []string) string {
	min := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, "\t "))
		if min < 0 || n < min {
			min = n
		}
	}
	if min <= 0 {
		return strings.Join(lines, "\n")
	}
	for i, l := range lines {
		if len(l) >= min {
			lines[i] = l[min:]
		}
	}
	return strings.Join(lines, "\n")
}

// ExprText returns the whitespace- and comment-insensitive token text of the
// expression or statement at pos, used for fingerprints. For a call site
// instruction pos is the position of the opening parenthesis.
func (p *Program) ExprText(pos token.Pos) string {
	f := p.File(pos)
	if f == nil {
		return ""
	}
	path, _ := astutil.PathEnclosingInterval(f, pos, pos)
	for _, n := range path {
		if c, ok := n.(*ast.CallExpr); ok && c.Lparen == pos {
			return p.normalize(c.Pos(), c.End())
		}
	}
	if start, end, ok := p.stmtRange(pos); ok {
		return p.normalize(start, end)
	}
	return ""
}

// BodyText returns the normalized token text of a function declaration or
// literal, used for the body fingerprint.
func (p *Program) BodyText(fn *ssa.Function) string { return p.BodyTextOf(fn.Syntax()) }

// BodyTextOf is BodyText for the syntax of a function.
func (p *Program) BodyTextOf(n ast.Node) string {
	if n == nil {
		return ""
	}
	return p.normalize(n.Pos(), n.End())
}

// normalize returns the tokens of [start,end) joined by single spaces,
// without comments.
func (p *Program) normalize(start, end token.Pos) string {
	tf := p.Fset.File(start)
	if tf == nil || end <= start {
		return ""
	}
	src := p.source(tf.Name())
	if src == nil {
		return ""
	}
	so, eo := tf.Offset(start), tf.Offset(end)
	if so < 0 || eo > len(src) || so >= eo {
		return ""
	}
	var s scanner.Scanner
	fs := token.NewFileSet()
	file := fs.AddFile("", fs.Base(), eo-so)
	s.Init(file, src[so:eo], nil, 0) // comments are skipped
	var buf bytes.Buffer
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON && lit == "\n" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		if lit != "" {
			buf.WriteString(lit)
		} else {
			buf.WriteString(tok.String())
		}
	}
	return buf.String()
}

// rawText returns the source text of [start, end) as written.
func (p *Program) rawText(start, end token.Pos) string {
	tf := p.Fset.File(start)
	if tf == nil || end <= start {
		return ""
	}
	src := p.source(tf.Name())
	so, eo := tf.Offset(start), tf.Offset(end)
	if src == nil || so < 0 || eo > len(src) || so >= eo {
		return ""
	}
	return string(src[so:eo])
}

// CondText returns the source text of the condition of a branch, given a
// position taken from inside the branch: the first instruction of the block
// it leads to. An "if" has no position of its own in go/ssa, and when the
// condition is a plain variable no instruction evaluates it.
func (p *Program) CondText(inside token.Pos) string {
	f := p.File(inside)
	if f == nil || !inside.IsValid() {
		return ""
	}
	path, _ := astutil.PathEnclosingInterval(f, inside, inside)
	for _, n := range path {
		switch s := n.(type) {
		case *ast.IfStmt:
			if s.Cond != nil {
				return p.rawText(s.Cond.Pos(), s.Cond.End())
			}
		case *ast.ForStmt:
			if s.Cond != nil {
				return p.rawText(s.Cond.Pos(), s.Cond.End())
			}
		}
	}
	return ""
}

// CallArgText returns the source text of the i-th argument (receiver
// excluded) of the call whose opening parenthesis is at lparen, and of the
// whole call.
func (p *Program) CallArgText(lparen token.Pos, i int) (arg, call string) {
	f := p.File(lparen)
	if f == nil || !lparen.IsValid() {
		return "", ""
	}
	path, _ := astutil.PathEnclosingInterval(f, lparen, lparen)
	for _, n := range path {
		if c, ok := n.(*ast.CallExpr); ok && c.Lparen == lparen {
			call = p.rawText(c.Pos(), c.End())
			if i >= 0 && i < len(c.Args) {
				arg = p.rawText(c.Args[i].Pos(), c.Args[i].End())
			}
			return arg, call
		}
	}
	return "", ""
}

// FuncSource returns the source of a function, cut at maxLines lines.
func (p *Program) FuncSource(fn *ssa.Function, maxLines int) string {
	return p.FuncSourceOf(fn.Syntax(), maxLines)
}

// FuncSourceOf is FuncSource for the syntax of a function.
func (p *Program) FuncSourceOf(n ast.Node, maxLines int) string {
	if n == nil {
		return ""
	}
	s := p.rawText(n.Pos(), n.End())
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "// ... (cut)")
	}
	return strings.Join(lines, "\n")
}

// ParamLabel describes a parameter of a function for a reader, such as
// "parameter 1 (escape bool)". Parameters count from 0, the receiver first.
func ParamLabel(fn *ssa.Function, i int) string {
	if i >= 0 && i < len(fn.Params) {
		prm := fn.Params[i]
		return fmt.Sprintf("parameter %d (%s %s)", i, prm.Name(), types.TypeString(prm.Type(), func(pkg *types.Package) string { return pkg.Name() }))
	}
	return fmt.Sprintf("parameter %d", i)
}
