package ir

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// The stream is JSON Lines. The first line is {"schema_version": ...}; then
// one line per function, and one line per implementations record.

type header struct {
	SchemaVersion string `json:"schema_version"`
}

type record struct {
	Kind   string           `json:"kind"`
	Func   *Func            `json:"func,omitempty"`
	Impls  *Implementations `json:"implementations,omitempty"`
	Global *Global          `json:"global,omitempty"`

	Text      *TextGroup `json:"text,omitempty"`
	CallText  *CallText  `json:"call_text,omitempty"`
	FuncText  *FuncText  `json:"func_text,omitempty"`
	LoadError *LoadError `json:"load_error,omitempty"`
}

const (
	recFunc   = "func"
	recImpls  = "implementations"
	recGlobal = "global"
	recText   = "text"
	recCall   = "call_text"
	recFunc2  = "func_text"
	recLoad   = "load_error"
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
		if err := enc.Encode(record{Kind: recFunc, Func: f}); err != nil {
			return err
		}
	}
	for i := range p.Impls {
		if err := enc.Encode(record{Kind: recImpls, Impls: &p.Impls[i]}); err != nil {
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
	return nil
}

// Read reads a stream and validates the program.
func Read(r io.Reader) (*Program, error) {
	sc := bufio.NewReaderSize(r, 1<<20)
	dec := json.NewDecoder(sc)
	dec.DisallowUnknownFields()
	var h header
	if err := dec.Decode(&h); err != nil {
		return nil, fmt.Errorf("ir: no header: %w", err)
	}
	if h.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("ir: unsupported schema %q (want %q)", h.SchemaVersion, SchemaVersion)
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
			p.Funcs = append(p.Funcs, rec.Func)
		case rec.Kind == recImpls && rec.Impls != nil:
			p.Impls = append(p.Impls, *rec.Impls)
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
		default:
			return nil, fmt.Errorf("ir: record %d: unknown record %q", n, rec.Kind)
		}
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("ir: %w", err)
	}
	return p, nil
}
