package ir

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// The stream is JSON Lines. The first line is {"schema_version": ...}; then
// one line per function, and one line per implementations record.

// SchemaError is the stream of an extractor whose version of the schema is not the one of this reader: the extractor is older, or newer, than
// rivus. The stream is not read: an older extractor does not write what the reader now expects, and nothing in the result would say so.
type SchemaError struct{ Got string }

func (e *SchemaError) Error() string {
	got := e.Got
	if got == "" {
		got = "(none)"
	}
	return fmt.Sprintf("ir: the stream is of schema %s, and this rivus reads %s: the extractor that wrote it is older or newer than rivus; build it again from the same source as rivus (a saved IR file, read with -ir, is written again with the extractor)", got, SchemaVersion)
}

type header struct {
	SchemaVersion string `json:"schema_version"`
}

type record struct {
	Kind   string           `json:"kind"`
	Func   *Func            `json:"func,omitempty"`
	Impls  *Implementations `json:"implementations,omitempty"`
	Global *Global          `json:"global,omitempty"`
	Type   *TypeInfo        `json:"type,omitempty"`
	Alias  *TypeAlias       `json:"type_alias,omitempty"`

	Text      *TextGroup `json:"text,omitempty"`
	CallText  *CallText  `json:"call_text,omitempty"`
	FuncText  *FuncText  `json:"func_text,omitempty"`
	LoadError *LoadError `json:"load_error,omitempty"`

	TestsSkipped *TestsSkipped `json:"tests_skipped,omitempty"`
	CallerMarks  bool          `json:"caller_marks,omitempty"`
	BodyNotRead  *BodyNotRead  `json:"body_not_read,omitempty"`
	FnSkipped    *FnSkipped    `json:"fn_skipped,omitempty"`
}

const (
	recFunc    = "func"
	recImpls   = "implementations"
	recGlobal  = "global"
	recType    = "type"
	recAlias   = "type_alias"
	recText    = "text"
	recCall    = "call_text"
	recFunc2   = "func_text"
	recLoad    = "load_error"
	recTests   = "tests_skipped"
	recMarks   = "caller_marks"
	recNoBody  = "body_not_read"
	recSkipped = "fn_skipped"
)

// Write writes the program as a stream. It does not validate: Validate is
// for the reader, and for the tests of an extractor.
func Write(w io.Writer, p *Program) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(header{SchemaVersion: SchemaVersion}); err != nil {
		return err
	}
	for _, f := range p.Funcs {
		restore := f.compact()
		err := enc.Encode(record{Kind: recFunc, Func: f})
		restore()
		if err != nil {
			return err
		}
	}
	for i := range p.Impls {
		if err := enc.Encode(record{Kind: recImpls, Impls: &p.Impls[i]}); err != nil {
			return err
		}
	}
	for i := range p.Types {
		if err := enc.Encode(record{Kind: recType, Type: &p.Types[i]}); err != nil {
			return err
		}
	}
	for i := range p.TypeAliases {
		if err := enc.Encode(record{Kind: recAlias, Alias: &p.TypeAliases[i]}); err != nil {
			return err
		}
	}
	for i := range p.Globals {
		if err := enc.Encode(record{Kind: recGlobal, Global: &p.Globals[i]}); err != nil {
			return err
		}
	}
	for i := range p.Texts {
		if err := enc.Encode(record{Kind: recText, Text: &p.Texts[i]}); err != nil {
			return err
		}
	}
	for i := range p.CallTexts {
		if err := enc.Encode(record{Kind: recCall, CallText: &p.CallTexts[i]}); err != nil {
			return err
		}
	}
	for i := range p.FuncTexts {
		if err := enc.Encode(record{Kind: recFunc2, FuncText: &p.FuncTexts[i]}); err != nil {
			return err
		}
	}
	for i := range p.LoadErrors {
		if err := enc.Encode(record{Kind: recLoad, LoadError: &p.LoadErrors[i]}); err != nil {
			return err
		}
	}
	if p.TestsSkipped != nil {
		if err := enc.Encode(record{Kind: recTests, TestsSkipped: p.TestsSkipped}); err != nil {
			return err
		}
	}
	for i := range p.BodiesNotRead {
		if err := enc.Encode(record{Kind: recNoBody, BodyNotRead: &p.BodiesNotRead[i]}); err != nil {
			return err
		}
	}
	for i := range p.FnSkipped {
		if err := enc.Encode(record{Kind: recSkipped, FnSkipped: &p.FnSkipped[i]}); err != nil {
			return err
		}
	}
	if p.CallerMarks {
		if err := enc.Encode(record{Kind: recMarks, CallerMarks: true}); err != nil {
			return err
		}
	}
	return nil
}

// Read reads a stream and validates the program.
func Read(r io.Reader) (*Program, error) {
	shared := interner{}
	p, err := decodeStream(r, func(f *Func) error {
		f.intern(shared)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("ir: %w", err)
	}
	return p, nil
}

// ReadFuncs reads a stream a function at a time: fn gets each function, validated, in the order of the stream, and the
// reader does not keep it. What is returned is the program without its functions (Funcs is nil; Files is the list that
// SourceFiles gives for the program with them). The program is checked as Read checks it. A function that fn does not
// keep is not held: the memory of the reader is the one line and the IDs of the functions.
func ReadFuncs(r io.Reader, fn func(*Func) error) (*Program, error) {
	seen := map[FuncID]bool{}
	files := fileSet{}
	p, err := decodeFuncs(r, func(f *Func) error {
		if seen[f.ID] {
			return fmt.Errorf("ir: function %s is defined twice", f.ID)
		}
		seen[f.ID] = true
		if err := f.Validate(); err != nil {
			return fmt.Errorf("ir: function %s: %w", f.ID, err)
		}
		files.addFunc(f)
		return fn(f)
	})
	if err != nil {
		return nil, err
	}
	for _, im := range p.Impls {
		if im.Method == "" {
			return nil, fmt.Errorf("ir: implementations without a method")
		}
	}
	files.addProgram(p)
	p.Files = files.list()
	return p, nil
}

// decodeStream is decodeFuncs that keeps the functions in the program.
func decodeStream(r io.Reader, each func(*Func) error) (*Program, error) {
	var funcs []*Func
	p, err := decodeFuncs(r, func(f *Func) error {
		if err := each(f); err != nil {
			return err
		}
		funcs = append(funcs, f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	p.Funcs = funcs
	return p, nil
}

// decodeFuncs decodes the records of a stream: the functions go to fn (each expanded), the others make the program.
func decodeFuncs(r io.Reader, fn func(*Func) error) (*Program, error) {
	sc := bufio.NewReaderSize(r, 1<<20)
	dec := json.NewDecoder(sc)
	dec.DisallowUnknownFields()
	var h header
	if err := dec.Decode(&h); err != nil {
		return nil, fmt.Errorf("ir: no header: %w", err)
	}
	if h.SchemaVersion != SchemaVersion {
		return nil, &SchemaError{Got: h.SchemaVersion}
	}
	p := &Program{}
	for n := 2; ; n++ {
		var rec record
		err := dec.Decode(&rec)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ir: record %d: %w", n, err)
		}
		switch {
		case rec.Kind == recFunc && rec.Func != nil:
			rec.Func.expand()
			if err := fn(rec.Func); err != nil {
				return nil, err
			}
		case rec.Kind == recImpls && rec.Impls != nil:
			p.Impls = append(p.Impls, *rec.Impls)
		case rec.Kind == recType && rec.Type != nil:
			p.Types = append(p.Types, *rec.Type)
		case rec.Kind == recAlias && rec.Alias != nil:
			p.TypeAliases = append(p.TypeAliases, *rec.Alias)
		case rec.Kind == recGlobal && rec.Global != nil:
			p.Globals = append(p.Globals, *rec.Global)
		case rec.Kind == recText && rec.Text != nil:
			p.Texts = append(p.Texts, *rec.Text)
		case rec.Kind == recCall && rec.CallText != nil:
			p.CallTexts = append(p.CallTexts, *rec.CallText)
		case rec.Kind == recFunc2 && rec.FuncText != nil:
			p.FuncTexts = append(p.FuncTexts, *rec.FuncText)
		case rec.Kind == recLoad && rec.LoadError != nil:
			p.LoadErrors = append(p.LoadErrors, *rec.LoadError)
		case rec.Kind == recTests && rec.TestsSkipped != nil:
			p.TestsSkipped = p.TestsSkipped.Add(rec.TestsSkipped)
		case rec.Kind == recNoBody && rec.BodyNotRead != nil:
			p.BodiesNotRead = append(p.BodiesNotRead, *rec.BodyNotRead)
		case rec.Kind == recSkipped && rec.FnSkipped != nil:
			p.FnSkipped = append(p.FnSkipped, *rec.FnSkipped)
		case rec.Kind == recMarks && rec.CallerMarks:
			p.CallerMarks = true
		default:
			return nil, fmt.Errorf("ir: record %d: unknown record %q", n, rec.Kind)
		}
	}
	return p, nil
}
