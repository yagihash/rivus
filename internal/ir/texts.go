package ir

// Texts cuts text from the source: what a reader is shown, and what the
// fingerprints are made of. The IR carries positions, not text; the text
// belongs to the language, because where a statement ends and what counts as
// the same expression are matters of its syntax. The extractor of a language
// provides the implementation: in the same process, over its own syntax tree.
type Texts interface {
	// Snippet is the statement that encloses the position, as the reader
	// would want to see it (the header of a compound statement).
	Snippet(Pos) string
	// Expr is the normalized text of the call expression at the position
	// (the opening parenthesis of a call), or of the enclosing statement. It
	// does not change with formatting or comments.
	Expr(Pos) string
	// Body is the normalized text of a function.
	Body(FuncID) string
	// Cond is the text of the condition of the branch that encloses a
	// position taken from inside it.
	Cond(Pos) string
	// CallArg is the text of the i-th argument (receiver excluded) of the
	// call at the position, and of the whole call.
	CallArg(Pos, int) (arg, call string)
	// FuncSource is the source of a function, cut at maxLines lines.
	FuncSource(FuncID, int) string
}

// SourceLines is the number of lines of a function the output shows. An
// extractor that writes the source into the stream cuts it at this length.
const SourceLines = 80
