// Package ir defines the neutral intermediate representation that every
// extractor produces and the core analyses (ARCHITECTURE 2, ADR-003, ADR-039).
//
// A function is a control flow graph of basic blocks. Values live in named
// local variables that may be assigned more than once: there are no φ
// functions, the core builds the SSA form (internal/ssabuild). Exceptions,
// async calls, defer and go are edges and flags, not instructions.
//
// The IR carries positions, not text: snippets are cut from the source by the
// core. It does carry the full content of constants.
package ir

import "fmt"

// SchemaVersion identifies the format of the stream.
const SchemaVersion = "rivus.ir/0"

type (
	// FuncID is the fully qualified name of a function.
	FuncID string
	// TypeID names a type; two values have the same type iff the IDs are equal.
	TypeID string
	// LocalID numbers a local variable in a function.
	LocalID int
	// BlockID numbers a block in a function.
	BlockID int
)

// NoLocal is the absence of a local, where one is optional.
const NoLocal LocalID = -1

// Pos is a range in a source file.
type Pos struct {
	File   string `json:"file,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Length int    `json:"length,omitempty"`
	// Line and Col (1-based) are for the reader of the stream; the offset is
	// what identifies the place.
	Line int `json:"line,omitempty"`
	Col  int `json:"col,omitempty"`
}

// IsZero reports whether the position is unknown.
func (p Pos) IsZero() bool { return p == Pos{} }

// Position is a position as the output shows it.
type Position struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Offset int    `json:"offset"`
}

func (p Position) String() string { return fmt.Sprintf("%s:%d", p.File, p.Line) }

// Position converts a position of the IR to the one the output shows.
func (p Pos) Position() Position {
	return Position{File: p.File, Line: p.Line, Column: p.Col, Offset: p.Offset}
}

// LoadError records something that could not be read: it maps to the
// missing_source terminal kind.
type LoadError struct {
	Package string `json:"package"`
	Message string `json:"message"`
}

// TypeClass is what the engine asks of a type; the structure of types is not
// carried (ADR-039).
type TypeClass string

const (
	ClassPointer   TypeClass = "pointer"   // the location of the value is what it points to
	ClassInterface TypeClass = "interface" // calls through it are dynamic
	ClassFunc      TypeClass = "func"
	// ClassContainer is a value that refers to elements stored elsewhere (a
	// slice, a map, a channel, a list): like a pointer, its location is what
	// it refers to. An array held by value is a struct.
	ClassContainer TypeClass = "container"
	ClassStruct    TypeClass = "struct" // an aggregate held by value: a struct, an array
	ClassBool      TypeClass = "bool"
	ClassString    TypeClass = "string"
	ClassNumber    TypeClass = "number"
	ClassOther     TypeClass = "other"
)

func (c TypeClass) valid() bool {
	switch c {
	case ClassPointer, ClassInterface, ClassFunc, ClassContainer, ClassStruct, ClassBool, ClassString, ClassNumber, ClassOther, "":
		return true
	}
	return false
}

// Local is a local variable of a function.
type Local struct {
	Name  string    `json:"name,omitempty"`
	Type  TypeID    `json:"type,omitempty"`
	Class TypeClass `json:"class,omitempty"`
}

// Param is a parameter. The receiver is parameter 0. The captured values of a
// closure follow the declared parameters.
type Param struct {
	Name  string    `json:"name,omitempty"`
	Type  TypeID    `json:"type,omitempty"`
	Class TypeClass `json:"class,omitempty"`
	// Display is the type as a reader writes it (package names, not import
	// paths).
	Display  string `json:"display,omitempty"`
	Captured bool   `json:"captured,omitempty"`
}

// Func is a function.
type Func struct {
	ID      FuncID   `json:"id"`
	Params  []Param  `json:"params,omitempty"`
	Results []TypeID `json:"results,omitempty"`
	Locals  []Local  `json:"locals,omitempty"`
	// Blocks[0] is the entry.
	Blocks []Block `json:"blocks"`
	// Roots lists the blocks where execution can begin: the entry, and for a
	// function with a deferred recover the block that runs after a panic.
	// The entry is always a root. A root has no predecessor.
	Roots    []BlockID `json:"roots,omitempty"`
	Pos      Pos       `json:"pos,omitempty"`
	Exported bool      `json:"exported,omitempty"`
	// Symbol is the name the rules use for the function: the declared
	// function or method (empty for an anonymous or a synthetic function).
	Symbol string `json:"symbol,omitempty"`
	// Recv is true when parameter 0 is the receiver of a method.
	Recv bool `json:"recv,omitempty"`
	// Dep marks a function of a dependency: it is there so that calls into
	// it can be followed, but it is not part of the code being analysed.
	Dep bool `json:"dep,omitempty"`
}

// EdgeKind says how control goes from a block to another.
type EdgeKind string

const (
	EdgeNormal    EdgeKind = "normal"
	EdgeTrue      EdgeKind = "true"
	EdgeFalse     EdgeKind = "false"
	EdgeException EdgeKind = "exception"
	// EdgeDefer leads to the code that runs deferred calls at the exit.
	EdgeDefer EdgeKind = "defer"
	// EdgeBack is a loop's back edge; it is an annotation on a normal edge.
	EdgeBack EdgeKind = "back"
)

// Edge is a successor of a block.
type Edge struct {
	To   BlockID  `json:"to"`
	Kind EdgeKind `json:"kind"`
}

// Block is a basic block: instructions, a terminator, and the edges out.
type Block struct {
	ID     BlockID `json:"id"`
	Instrs []Instr `json:"instrs,omitempty"`
	Term   Term    `json:"term"`
	// Succs are the edges out. The terminator decides which are required: a
	// jump has one normal (or back) edge, a branch has a true and a false
	// edge. Exception edges may be added to a block that ends in a call, or
	// in a throw; they are taken instead of the normal edge.
	Succs []Edge `json:"succs,omitempty"`
}

// TermKind is the kind of a terminator.
type TermKind string

const (
	TermJump        TermKind = "jump"
	TermBranch      TermKind = "branch"
	TermReturn      TermKind = "return"
	TermThrow       TermKind = "throw"
	TermUnreachable TermKind = "unreachable"
)

// Term ends a block.
type Term struct {
	Kind TermKind `json:"kind"`
	// Cond is the condition of a branch: a local that holds it. CondPos is
	// the source range of the condition, from which a readable form is cut.
	Cond    LocalID `json:"cond,omitempty"`
	CondPos Pos     `json:"cond_pos,omitempty"`
	// Values are the returned values (return) or the thrown value (throw).
	Values []LocalID `json:"values,omitempty"`
	Pos    Pos       `json:"pos,omitempty"`
}

// Op is an instruction.
type Op string

const (
	OpConst          Op = "const"
	OpParam          Op = "param"
	OpCopy           Op = "copy"
	OpUnop           Op = "unop"
	OpBinop          Op = "binop"
	OpConvert        Op = "convert"
	OpTypeAssert     Op = "type_assert"
	OpAlloc          Op = "alloc"
	OpLoad           Op = "load"
	OpStore          Op = "store"
	OpAddrOf         Op = "addr_of"
	OpFieldLoad      Op = "field_load"
	OpFieldStore     Op = "field_store"
	OpIndexLoad      Op = "index_load"
	OpIndexStore     Op = "index_store"
	OpGlobalLoad     Op = "global_load"
	OpGlobalStore    Op = "global_store"
	OpMakeClosure    Op = "make_closure"
	OpCall           Op = "call"
	OpExceptionValue Op = "exception_value"
	OpUnsupported    Op = "unsupported"
)

// ConstKind is the kind of a constant.
type ConstKind string

const (
	ConstString ConstKind = "string"
	ConstInt    ConstKind = "int"
	ConstFloat  ConstKind = "float"
	ConstBool   ConstKind = "bool"
	ConstNil    ConstKind = "nil"
)

// Const is a literal. A string keeps its whole content. If an extractor must
// cut it, it says so: Truncated is true.
type Const struct {
	Kind      ConstKind `json:"kind"`
	Value     string    `json:"value,omitempty"`
	Truncated bool      `json:"truncated,omitempty"`
}

// StepKind is the kind of a step in an access path.
type StepKind string

const (
	StepField StepKind = "field"
	StepIndex StepKind = "index"
	StepDeref StepKind = "deref"
)

// Step is one step of the path of an addr_of: x.f[i] is field f, then index.
// The index value is not recorded: the engine does not distinguish elements.
type Step struct {
	Kind  StepKind `json:"kind"`
	Field string   `json:"field,omitempty"`
}

// CalleeKind says how the callee is known.
type CalleeKind string

const (
	// CalleeStatic: the function is known; Symbol names it. All of Args are
	// the arguments (the receiver first for a method).
	CalleeStatic CalleeKind = "static"
	// CalleeDynamic: a call through an interface or a virtual method. Symbol
	// is the method; Candidates are the implementations the extractor knows.
	// Args[0] is the receiver.
	CalleeDynamic CalleeKind = "dynamic"
	// CalleeValue: a call of a function value. Args[0] is the value.
	CalleeValue CalleeKind = "value"
)

// Callee is the target of a call.
type Callee struct {
	Kind CalleeKind `json:"kind"`
	// Func is the function a static call reaches (empty for a builtin).
	Func FuncID `json:"func,omitempty"`
	// Symbol is the name the rules use: for a static call, the declared
	// function or method (empty for an anonymous function); for a dynamic
	// call, the interface or virtual method; for a builtin, "builtin.<name>".
	Symbol string `json:"symbol,omitempty"`
	// Recv is true when Args[0] of a static call is the receiver of the
	// method (a dynamic call always has the receiver first).
	Recv       bool     `json:"recv,omitempty"`
	Candidates []FuncID `json:"candidates,omitempty"`
}

// Instr is an instruction. Which fields are used depends on Op (see Validate
// for the exact shape of each).
type Instr struct {
	Op Op `json:"op"`
	// Dsts are the locals written. Every op writes at most one, except a
	// call (one per result) and a type_assert (the value, and the success
	// flag when the assertion is of the comma-ok form).
	Dsts []LocalID `json:"dsts,omitempty"`
	Args []LocalID `json:"args,omitempty"`

	Const *Const `json:"const,omitempty"`
	// Operator is the operator of a unop or a binop, as the language writes
	// it. For a convert it names the kind of conversion: "convert" (a value
	// of another type), "make_interface", "change_interface" or
	// "slice_to_array_pointer".
	Operator string `json:"operator,omitempty"`
	// Index is the position of a param.
	Index int `json:"index,omitempty"`
	// Type is the type of an alloc, a convert or a type_assert. Class is the
	// class of the type an alloc allocates (the value it holds), which tells
	// what the zero value of a promoted variable is.
	Type  TypeID    `json:"type,omitempty"`
	Class TypeClass `json:"class,omitempty"`
	// Field is the field of a field_load or a field_store.
	Field string `json:"field,omitempty"`
	// Path is the path of an addr_of.
	Path []Step `json:"path,omitempty"`
	// Global is the variable of a global_load, a global_store, or an addr_of
	// that takes the address of a global (no argument then).
	Global string `json:"global,omitempty"`
	// Func is the function of a make_closure.
	Func FuncID `json:"func,omitempty"`
	// View marks a copy that shares memory with its source (a slice of an
	// array): the elements are the same elements.
	View bool `json:"view,omitempty"`
	// Folded marks a field or index access that went through an address: a
	// load or a store of x.f or x[i], where the address computation was
	// folded into the access.
	Folded bool `json:"folded,omitempty"`
	// Keyed marks an index access by key: a lookup in a map or a string, or
	// a store into a map. The index is a key, not a position.
	Keyed bool `json:"keyed,omitempty"`

	Callee *Callee `json:"callee,omitempty"`
	// Go, Defer and Await are the flags of a call.
	Go    bool `json:"go,omitempty"`
	Defer bool `json:"defer,omitempty"`
	Await bool `json:"await,omitempty"`

	// Reason says why an extractor could not express a construct
	// (unsupported).
	Reason string `json:"reason,omitempty"`

	Pos Pos `json:"pos,omitempty"`
	// AddrPos is the position of the address computation that a field or an
	// index access folded into itself (&x.f in *&x.f).
	AddrPos Pos `json:"addr_pos,omitempty"`
	// ArgPos are the source ranges of the arguments of a call, when the
	// extractor knows them (parallel to Args, or empty).
	ArgPos []Pos `json:"arg_pos,omitempty"`
}

// Implementations lists the known implementations of an interface method:
// the answer to the question "which functions can this call reach", for the
// rules that apply to every implementation (match: implementations).
type Implementations struct {
	Method string   `json:"method"`
	Impls  []FuncID `json:"impls"`
}

// Program is what an extractor produces: functions, and the facts about the
// whole program that the core cannot get from a function alone.
type Program struct {
	Funcs []*Func           `json:"funcs"`
	Impls []Implementations `json:"implementations,omitempty"`
	// Globals are the package variables the functions use.
	Globals []Global `json:"globals,omitempty"`
}

// Global is a package variable (or a static): where it is declared.
type Global struct {
	Name string `json:"name"`
	Type TypeID `json:"type,omitempty"`
	Pos  Pos    `json:"pos,omitempty"`
}
