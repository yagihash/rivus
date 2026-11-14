package gossa

import (
	"bytes"
	"go/ast"
	"go/scanner"
	"go/token"
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
func (p *Program) BodyText(fn *ssa.Function) string {
	n := fn.Syntax()
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
