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

import (
	"fmt"
	"sort"
	"strings"
)

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

// SyntaxErrorPrefix starts the message of the load error that an extractor writes for a file that was read on a tree
// that the parser recovered (a syntax error), with the file as the package and unread set. The core takes such a file
// as one whose findings are not clear (taint.Engine.recoveredFiles). The Dart extractor writes it
// (extractor/dart/lib/extract.dart, "syntax_error:"); a test holds the two to each other (cmd/rivus/dart_syntax_test.go).
const SyntaxErrorPrefix = "syntax_error:"

// DependencyNotLinkedPrefix starts the message of the load error that the Dart extractor writes (unread set, with the
// directory of the dependency as the package) for a dependency file that is not read as the calls to it name it: it is
// not resolved to a package: URI, or another package of the tree reads another version of the same dependency, so the
// functions have the same IDs (extractor/dart/lib/extract.dart, "dependency_not_linked"). A test holds the two to each
// other (cmd/rivus/dart_depcontext_test.go).
const DependencyNotLinkedPrefix = "dependency_not_linked"

// DependencyNotAllowedPrefix starts the message of the load error that the Dart extractor writes (unread set, with the
// directory of the dependency as the package) for a dependency that is outside the places that are trusted (the
// project, the pub cache, the SDKs and -allow-deps-under): it is not read, and its functions are not analysed
// (extractor/dart/lib/extract.dart, "dependency_not_allowed"). A test holds the two to each other (cmd/rivus/dart_test.go).
const DependencyNotAllowedPrefix = "dependency_not_allowed"

// PackageConfigMissingPrefix and PackageConfigOldPrefix start the messages of the load errors that the Dart extractor writes
// (unread set, with the directory of the package) for a package that `dart pub get` was not run in, or was run in before a
// dependency was added: the imports of other packages do not resolve (extractor/dart/lib/extract.dart,
// _missingPackageConfigs). A test holds the two to each other (cmd/rivus/dart_packageconfig_test.go).
const (
	PackageConfigMissingPrefix = "package_config.json is not there"
	PackageConfigOldPrefix     = "package_config.json is old"
)

// LoadError records something that could not be read: it maps to the
// missing_source terminal kind.
type LoadError struct {
	Package string `json:"package"`
	Message string `json:"message"`
	// Unread says that the package was read and not built (a type error in it, or in a package it imports): what it
	// holds is not in the analysis. Functions is the number of its functions (with a body) that are not.
	Unread    bool `json:"unread,omitempty"`
	Functions int  `json:"unread_functions,omitempty"`
	// Func is the function that holds what the message says (a compile error of Dart: the declaration around it; a
	// closure of it counts as it): the findings that go through the function carry the message (missing_source).
	Func string `json:"func,omitempty"`
}

// BodyNotRead is a unit (a Go package, `package:name` of Dart) whose functions the extractor left without a body.
type BodyNotRead struct {
	Unit      string `json:"unit"`
	Functions int    `json:"functions"`
}

// TestsSkipped says what an extractor did not read because the build of the project does not ship it
// (ADR-062): the test files, the tests, examples and benchmarks of a crate, the dependencies that are
// declared for development only. The same shape for every language.
type TestsSkipped struct {
	// Files are the test files that were not read (Go: _test.go; Dart: the files of test/, integration_test/,
	// benchmark/, tool/ and example/ of a package).
	Files int `json:"files"`
	// Targets are the test, example and benchmark targets of a crate that were not built (Rust).
	Targets int `json:"targets,omitempty"`
	// DevDependencies are the dependencies declared for development only, and not reached from the code that is shipped.
	DevDependencies int `json:"dev_dependencies"`
	// DevDependencyNames names them (up to 50).
	DevDependencyNames []string `json:"dev_dependency_names,omitempty"`
	// Notes are what the reader of the result should know (a dependency declared for development that the code uses).
	Notes []string `json:"notes,omitempty"`
	// SkippedDirs are the directories of the project that an extractor did not read because of where they are (Dart:
	// a hidden directory, and `build` at the top), SkippedDirNames names some of them (up to 5). They are not tests:
	// code in them may be shipped. The result says them in summary.skipped_dirs (not in stats.tests_skipped).
	SkippedDirs     int      `json:"skipped_dirs,omitempty"`
	SkippedDirNames []string `json:"skipped_dir_names,omitempty"`
	// OutsideBuildFiles are the files of the project that the build constraints leave out of this configuration (Go:
	// `//go:build ignore`, `_windows.go` on another system), OutsideBuildFileNames names some of them (up to 5). They are
	// not tests: the result says them in summary.outside_configuration (not in stats.tests_skipped).
	OutsideBuildFiles     int      `json:"outside_build_files,omitempty"`
	OutsideBuildFileNames []string `json:"outside_build_file_names,omitempty"`
}

// Add adds the counts of another language.
func (t *TestsSkipped) Add(o *TestsSkipped) *TestsSkipped {
	if o == nil {
		return t
	}
	if t == nil {
		c := *o
		return &c
	}
	t.Files += o.Files
	t.Targets += o.Targets
	t.DevDependencies += o.DevDependencies
	t.DevDependencyNames = append(t.DevDependencyNames, o.DevDependencyNames...)
	t.Notes = append(t.Notes, o.Notes...)
	t.SkippedDirs += o.SkippedDirs
	t.SkippedDirNames = append(t.SkippedDirNames, o.SkippedDirNames...)
	if len(t.SkippedDirNames) > 5 {
		t.SkippedDirNames = t.SkippedDirNames[:5]
	}
	t.OutsideBuildFiles += o.OutsideBuildFiles
	t.OutsideBuildFileNames = append(t.OutsideBuildFileNames, o.OutsideBuildFileNames...)
	if len(t.OutsideBuildFileNames) > 5 {
		t.OutsideBuildFileNames = t.OutsideBuildFileNames[:5]
	}
	return t
}

// Empty is true when nothing was left out.
func (t *TestsSkipped) Empty() bool {
	return t == nil || (t.Files == 0 && t.Targets == 0 && t.DevDependencies == 0)
}

// Describe is the line that says it.
func (t *TestsSkipped) Describe() string {
	var parts []string
	if t.Files > 0 {
		parts = append(parts, fmt.Sprintf("%d test files", t.Files))
	}
	if t.Targets > 0 {
		parts = append(parts, fmt.Sprintf("%d test, example and benchmark targets", t.Targets))
	}
	if t.DevDependencies > 0 {
		parts = append(parts, fmt.Sprintf("%d dependencies declared for development", t.DevDependencies))
	}
	return strings.Join(parts, ", ") + " were not read (--tests reads them: the build does not ship them)"
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
	Roots []BlockID `json:"roots,omitempty"`
	Pos   Pos       `json:"pos,omitempty"`
	// File is the file the positions of the function are in, unless one
	// says otherwise: a position without a file has this one. It keeps the
	// stream small (a path is most of a position).
	File     string `json:"file,omitempty"`
	Exported bool   `json:"exported,omitempty"`
	// Entry marks where the program starts: main, and the initialisation of a
	// package. What no entry reaches (a helper that only the tests use, a
	// mock) is not part of the program that runs.
	Entry bool `json:"entry,omitempty"`
	// Init marks the initializer of a variable (the function that stores its value, named
	// "<global>$init"): it runs when the variable is first read, so it is an entry, but it
	// is not where a program starts: a library that has only these still starts from its
	// exported functions (yagihash/rivus #60).
	Init bool `json:"init,omitempty"`
	// Symbol is the name the rules use for the function: the declared
	// function or method (empty for an anonymous or a synthetic function).
	Symbol string `json:"symbol,omitempty"`
	// Recv is true when parameter 0 is the receiver of a method.
	Recv bool `json:"recv,omitempty"`
	// Dep marks a function of a dependency: it is there so that calls into
	// it can be followed, but it is not part of the code being analysed.
	Dep bool `json:"dep,omitempty"`
	// Private marks a function that the code of its own crate only can call: the source says it has no
	// `pub` (Rust). Its callers are all among the functions that are read, wherever the program starts.
	// No mark says nothing (an extractor that does not know, or a function that may be called from outside).
	Private bool `json:"private,omitempty"`
	// ExternalEntry marks a function that another program can call by its symbol (`#[no_mangle]`,
	// `#[export_name]`, `extern "C"`, anything of a crate built as a library of C): a program that has
	// one is called from outside, so its callers are not all read (Rust).
	ExternalEntry bool `json:"external_entry,omitempty"`
	// Overrides are the symbols of the methods of the supertypes that this method overrides or
	// implements (an extractor lists them, outside the language's own library). One of them
	// that no function of the code being analysed has as its symbol is a method that a
	// framework declares: this one is called by the framework, and the program does not
	// start there (Program.FrameworkCallbacks, ADR-073). Dart only for now.
	Overrides []string `json:"overrides,omitempty"`
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

// ValueCand is a function a call of a function value may reach.
type ValueCand struct {
	ID FuncID
	// Symbol is the name the rules use for it: the symbol of the function if
	// the program has it, else its ID (a function without a body is known by
	// the name it is called by).
	Symbol string
	// Recv is true when the first argument of the call is the receiver of the
	// method (a method taken as a value of a function: `(*T).M`).
	Recv bool
	// Param is the parameter of the function that holds the call, when the
	// function reaches the value only through that parameter: the callers
	// decide which of the candidates is called (a guard follows the flow).
	// -1 when the function is chosen in the function itself.
	Param int
	// Bind is the instruction that made the closure, when it was made in the
	// function that holds the call and captured values (they are values of that
	// function): the candidate is called with them.
	Bind *Binding
}

// Binding is the place of an instruction in a function.
type Binding struct{ Block, Instr int }

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
	// ValueCands are the functions a call of a function value may reach, with
	// the names the rules use for them. They are not part of the stream: the
	// core fills them in (internal/valuecalls), so that the sites and the
	// graph see the same ones.
	ValueCands []ValueCand `json:"-"`
	// ValueLimit says why a call of a function value has no ValueCands although the functions
	// were found: rivus left them out because of its own limit, which is not the same as the
	// function not being known. "candidate_limit": more functions than the limit flow to the
	// value (ValueFound of them); "caller_depth_limit": the callers were not followed far
	// enough to find any. Empty when nothing was cut.
	ValueLimit string `json:"-"`
	ValueFound int    `json:"-"`
	// ValueLimitParams are, when ValueLimit is "candidate_limit", the parameters of the function that the called value
	// came through (the one parameter, when it is one): what is passed there is called, and not followed. Not in the stream.
	ValueLimitParams []int `json:"-"`
	// ValueUnresolvedParams are, when the call has no candidate and the callers did not give one, the parameter of the
	// function that the called value came through, also through a field or an element of what the parameter holds: what a
	// caller passes there is called, and which function it is, is not known. Not in the stream.
	ValueUnresolvedParams []int `json:"-"`
	// ValueWhy is, when ValueLimit is "partial" (ValueCands are set, ValueFound of them), why not
	// every caller's value could be followed: the causes, sorted.
	ValueWhy []string `json:"-"`
	// NarrowedFrom is the number of candidates a call through Fn::call (Rust) had before the
	// candidates were narrowed to the functions that flow to the receiver (internal/valuecalls); 0 when
	// they were not. NarrowBasis is the assumption the narrowing rests on ("closed_world_entry", or empty
	// when the receiver was made in the function itself), and NarrowCallers says where the receiver came
	// from, for the text of the uncertainty.
	NarrowedFrom int    `json:"-"`
	NarrowBasis  string `json:"-"`
	// CandParam is, for the candidates that came to the receiver only through one parameter of the
	// function that holds the call, that parameter: the callers know which function they passed.
	CandParam  map[FuncID]int `json:"-"`
	NarrowFrom string         `json:"-"`
	// ValueSupplemented is set when the ValueCands were not found by valuecalls but by the analysis of the function values
	// (-resolve-func-values, docs/DESIGN-callouts.md 5.6): the functions are those that the code of the project makes
	// (a function made in a dependency is not followed), so the candidates are not known to be all of them. The call keeps
	// its uncertainty. ValueRoundsLimit is set too when the last round still added candidates (the rounds were cut).
	// Not in the stream.
	ValueSupplemented bool `json:"-"`
	ValueRoundsLimit  bool `json:"-"`
	// FFI marks a call of a foreign function (an extern "C" function of Rust): the
	// call leaves the language, and what happens in it is not code that can be read.
	FFI bool `json:"ffi,omitempty"`
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
	// ArgNames are the names of the named parameters that the operands of a call are for (parallel to
	// Args, or empty; "" for an operand that is not a named parameter, and for the receiver). A language
	// with named arguments (Dart) puts them after the positional ones, by name, so the position of one
	// depends on the declaration; a rule that says `named: kind` finds it by this name. It names the
	// parameters of the callee, so an argument that the call leaves out still has its name (and holds
	// nil). Empty for a language without named arguments, and for a callee the extractor could not resolve.
	ArgNames []string `json:"arg_names,omitempty"`
}

// Implementations lists the known implementations of an interface method:
// the answer to the question "which functions can this call reach", for the
// rules that apply to every implementation (match: implementations).
type Implementations struct {
	Method string   `json:"method"`
	Impls  []FuncID `json:"impls"`
}

// TypeInfo says what a class is a subtype of: the types it extends,
// implements and mixes in, directly. The type IDs are those of the
// parameters (Param.Type).
type TypeInfo struct {
	ID     TypeID   `json:"id"`
	Supers []TypeID `json:"supers,omitempty"`
}

// Program is what an extractor produces: functions, and the facts about the
// whole program that the core cannot get from a function alone.
type Program struct {
	Funcs []*Func           `json:"funcs"`
	Impls []Implementations `json:"implementations,omitempty"`
	// Types are the supertypes of the classes the program has, for the rules
	// that speak of "every subclass of" (extends:).
	Types []TypeInfo `json:"types,omitempty"`
	// Globals are the package variables the functions use.
	Globals []Global `json:"globals,omitempty"`
	// TypeAliases are the names that stand for another type (Go's `type A = B`).
	// A type in the IR keeps the name the code used; a rule that names the
	// alias and one that names the type behind it both match (ADR-083).
	TypeAliases []TypeAlias `json:"type_aliases,omitempty"`
	// The texts an extractor in another process writes, because the core
	// cannot ask it for them (see StreamTexts).
	Texts     []TextGroup `json:"texts,omitempty"`
	CallTexts []CallText  `json:"call_texts,omitempty"`
	FuncTexts []FuncText  `json:"func_texts,omitempty"`
	// LoadErrors are what the extractor could not read.
	LoadErrors []LoadError `json:"load_errors,omitempty"`
	// CallerMarks says that the extractor marks the functions that the code of their crate only can call (Func.Private)
	// and the functions that another program can call by their symbol (Func.ExternalEntry): a function without a mark
	// is then one that may be called from outside. An extractor that does not say so (an older one) leaves no
	// function private, and the program is not read as a closed world: nothing is known of what calls it.
	CallerMarks bool `json:"caller_marks,omitempty"`
	// TestsSkipped is what was not read because the build does not ship it (ADR-062).
	TestsSkipped *TestsSkipped `json:"tests_skipped,omitempty"`
	// BodiesNotRead is what the extractor did not read because it was told the models stand for the package
	// (docs/DESIGN-package-models.md 2): per unit, how many functions it has that were left without a body. Empty
	// when the extractor was told no such package.
	BodiesNotRead []BodyNotRead `json:"bodies_not_read,omitempty"`

	// Files are the source files named by the positions, set by the reader
	// before the functions of dependencies that nothing reaches are dropped
	// (SourceFiles). Not part of the stream.
	Files []string `json:"-"`

	// ValuesResolved is set when the calls of function values have their
	// ValueCands (internal/valuecalls).
	ValuesResolved bool `json:"-"`
	// Narrowing counts what the narrowing of the candidates of Fn::call did (internal/valuecalls).
	Narrowing *NarrowStats `json:"-"`
}

// Kinds of text.
const (
	TextSnippet = "snippet" // the statement around a position
	TextExpr    = "expr"    // the normalized expression at a position
	TextCond    = "cond"    // the condition of the branch around a position
)

// TextGroup is one text and every position it is the text of. Many
// positions share a statement, so the text is written once.
type TextGroup struct {
	Kind    string `json:"kind"`
	File    string `json:"file"`
	Text    string `json:"text"`
	Offsets []int  `json:"offsets"`
}

// CallText is the text of a call as written, and of each of its arguments
// (the receiver is not counted).
type CallText struct {
	File   string   `json:"file"`
	Offset int      `json:"offset"`
	Call   string   `json:"call,omitempty"`
	Args   []string `json:"args,omitempty"`
}

// FuncText is the normalized body of a function (what the body fingerprint is
// made of) and its source, cut at a number of lines.
type FuncText struct {
	Func   FuncID `json:"func"`
	Body   string `json:"body,omitempty"`
	Source string `json:"source,omitempty"`
}

// Global is a package variable (or a static): where it is declared.
type Global struct {
	Name string `json:"name"`
	Type TypeID `json:"type,omitempty"`
	Pos  Pos    `json:"pos,omitempty"`
}

// SourceFiles lists the files the program's positions name, sorted: the
// source files an extractor read that left a trace in the IR. A function that
// PruneDeps drops is listed too when this is called before it.
func (p *Program) SourceFiles() []string {
	set := map[string]bool{}
	add := func(f string) {
		if f != "" {
			set[f] = true
		}
	}
	for _, f := range p.Funcs {
		if f == nil {
			continue
		}
		add(f.File)
		add(f.Pos.File)
		for bi := range f.Blocks {
			b := &f.Blocks[bi]
			for ii := range b.Instrs {
				in := &b.Instrs[ii]
				add(in.Pos.File)
				add(in.AddrPos.File)
				for _, a := range in.ArgPos {
					add(a.File)
				}
			}
			add(b.Term.Pos.File)
			add(b.Term.CondPos.File)
		}
	}
	for _, g := range p.Globals {
		add(g.Pos.File)
	}
	for _, t := range p.Texts {
		add(t.File)
	}
	for _, t := range p.CallTexts {
		add(t.File)
	}
	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// The reasons of Callee.ValueLimit.
const (
	ValueCandidateLimit = "candidate_limit"
	// ValueCandidateMost is the number of candidates beyond which a call of a function value is left unresolved (ADR-050;
	// 8 until 2026-11-22, then 32: measured, and the cost did not change on the benches).
	ValueCandidateMost    = 32
	ValueCallerDepthLimit = "caller_depth_limit"
	// ValuePartial: some functions were found (they are ValueCands) but not every caller's value
	// could be followed, so the candidates are not known to be all of them.
	ValuePartial = "partial"
)

// The causes in Callee.ValueWhy.
const (
	ValueWhyUntraced = "untraced_value" // a caller passes a value that is not a function or a copy of one (a field, a result, a global, ...)
	ValueWhyArgs     = "missing_argument"
	ValueWhyNoForm   = "no_ssa_form"
	ValueWhyDepth    = "caller_depth"
)

// ValueLimitDetail is the text of the uncertainty of a call of a function value that rivus gave up
// on because of a limit of its own (nothing is said when ValueLimit is empty).
func (c *Callee) ValueLimitDetail() string {
	switch c.ValueLimit {
	case ValueCandidateLimit:
		return fmt.Sprintf("candidate_limit: the function value may be any of %d functions, more than the %d that rivus keeps; none is followed (the functions are known: this is rivus' limit, not the code's)", c.ValueFound, ValueCandidateMost)
	case ValuePartial:
		return fmt.Sprintf("partial: the callers that pass the function value were followed only in part (%s); %d functions were found and are followed, others may be called (rivus' limit: the candidates are not known to be all of them)", strings.Join(c.ValueWhy, ", "), c.ValueFound)
	case ValueCallerDepthLimit:
		return "caller_depth_limit: the callers that pass the function value were followed 3 calls up and no function was found; the value may be one beyond that (rivus' limit)"
	}
	return ""
}

// ValueSupplementedDetail is the text of the uncertainty of a call whose candidates the analysis of the function values
// gave (empty when it did not). It starts with the name of the cause, as ValueLimitDetail does.
func (c *Callee) ValueSupplementedDetail() string {
	if !c.ValueSupplemented {
		return ""
	}
	return fmt.Sprintf("func_values_supplemented: the %d candidate(s) were found by following the function values that the code of the project makes (-resolve-func-values); functions made in dependencies are not followed, so the candidates are only functions made in the code of the project, and the function value may be others too", len(c.ValueCands))
}

// ValueRoundsDetail is the text of the uncertainty of a call whose candidates were still growing when the rounds of the
// analysis of the function values were cut (empty when they were not).
func (c *Callee) ValueRoundsDetail() string {
	if !c.ValueRoundsLimit {
		return ""
	}
	return "func_values_rounds: the candidates of the function value were still growing in the last round of the analysis of the function values (-resolve-func-values) that finished, and the rounds stopped there (the limit of the rounds, or the time limit -timeout); more functions may be called (rivus' limit)"
}

// NarrowStats is what the narrowing of the candidates of the calls of Rust closures did: the calls that were
// narrowed (with the candidates before and after, summed), and, for the others, why not.
type NarrowStats struct {
	Narrowed      int `json:"narrowed"`
	CandidatesOld int `json:"candidates_before"`
	CandidatesNew int `json:"candidates_after"`
	// The calls that stayed as they were: the callers of a function may be outside what is read, the
	// receiver comes from something that is not followed, or a limit of rivus was reached.
	OpenCallers   int `json:"open_callers,omitempty"`
	UnknownOrigin int `json:"unknown_origin,omitempty"`
	Limit         int `json:"limit,omitempty"`
}

// BasisClosedWorldEntry is the assumption of a narrowing by the callers of a function: the program has a
// start, and what it can call from outside is nothing that is read, so the callers that are read are all.
const BasisClosedWorldEntry = "closed_world_entry"

// TypeAlias says that the type Name is another name of Target.
type TypeAlias struct {
	Name   TypeID `json:"name"`
	Target TypeID `json:"target"`
}

// CanonType returns the aliases of the program as a TypeCanon: Canon gives the type behind the
// aliases of a type name, so two names of one type give one result. The aliases are resolved wherever
// they stand in the type, not only at its head: under the pointer and slice prefixes, and inside
// `map[string]Alias`, `Foo[Alias]`, `[]*Alias`, a function type (ADR-083). Without aliases it is the
// identity. It is a value with a method, not a func value: a closure handed to a helper that calls it
// is a capture rivus does not follow (capture_unbound).
func (p *Program) CanonType() TypeCanon {
	m := make(TypeCanon, len(p.TypeAliases))
	for _, a := range p.TypeAliases {
		m[string(a.Name)] = string(a.Target)
	}
	return m
}

// TypeCanon maps an alias name to its target.
type TypeCanon map[string]string

// Canon is the type behind the aliases of s.
func (m TypeCanon) Canon(s string) string {
	if len(m) == 0 {
		return s
	}
	return m.canon(s, 0)
}

// canon resolves the aliases in the names of s: the names are what the delimiters of a type leave.
func (m TypeCanon) canon(s string, depth int) string {
	var b strings.Builder
	changed := false
	for i := 0; i < len(s); {
		if typeDelim(s[i]) {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i
		for j < len(s) && !typeDelim(s[j]) {
			j++
		}
		tok := s[i:j]
		if t, ok := m[tok]; ok && t != tok && depth < 8 {
			// the target may itself be an alias, or have one in it
			tok = m.canon(t, depth+1)
			changed = true
		}
		b.WriteString(tok)
		i = j
	}
	if !changed {
		return s
	}
	return b.String()
}

// typeDelim reports whether c separates the names in a type written as a string: the prefixes (*, []), the
// brackets of a map, an array and the type arguments, and the parentheses and commas of a function type.
func typeDelim(c byte) bool {
	switch c {
	case '*', '[', ']', '(', ')', ',', ' ', '{', '}', ';':
		return true
	}
	return false
}
