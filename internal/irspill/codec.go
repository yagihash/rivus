// Package irspill is the way to hold the IR of a function outside the memory: a compact binary record of one
// ir.Func (this file), and a store that puts the records in a file that has no name (store.go).
//
// The record keeps what ir.Func keeps in the stream (the fields with `json:"-"` are not part of it: the core fills
// them in later). A nil slice and an empty one are not told apart, as in the JSON stream.
//
// The constants of the strings of the code (up to 64 KB each; a secret that is in the source is one of them) are
// in the records, so they reach the disk, in a file that has no name (store.go). The reader is the code of this
// package only.
//
// Record: length (4 bytes, of what follows), CRC-32 (Castagnoli) of the body (4 bytes), body. Body: the strings of the
// function (a count, then each as a length and bytes; the number 0 is "" and is not in the list), then the function.
// Numbers are varints (signed ones zig-zag). An instruction is the op (one byte), a mask of the fields it has (the
// common ones are in the low 7 bits, so that a load or a store takes one byte), and the fields. A position is the file
// (0 none, 1 the one of the position before, else the string number + 2), then the offset and the line as the
// difference from the position before, then the length and the column.
package irspill

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"math"

	"github.com/yagihash/rivus/internal/ir"
)

// MaxRecord is the most bytes of the body of one record. The reader refuses a longer one, before it makes
// the memory for it.
const MaxRecord = 256 << 20

// headerLen is the length of the length and the CRC in front of a body.
const headerLen = 8

var (
	// ErrCorrupt is the error of a record that is not what the writer made.
	ErrCorrupt = errors.New("irspill: corrupt record")
	crcTable   = crc32.MakeTable(crc32.Castagnoli)
)

// The masks of an instruction. The low 7 bits are the fields that most instructions have.
const (
	mDsts uint32 = 1 << iota
	mArgs
	mPos
	mType
	mField
	mConst
	mCallee
	mOperator
	mIndex
	mClass
	mPath
	mGlobal
	mFunc
	mView
	mFolded
	mKeyed
	mCharRead
	mGo
	mDefer
	mAwait
	mReason
	mAddrPos
	mArgPos
	mArgNames
	mMax // the mask is below this
)

// The enums: 0 is the empty string, 255 says that the string follows (a value no extractor writes today, so that
// the reader does not refuse what a later extractor writes).
var (
	ops       = []ir.Op{ir.OpConst, ir.OpParam, ir.OpCopy, ir.OpUnop, ir.OpBinop, ir.OpConvert, ir.OpTypeAssert, ir.OpAlloc, ir.OpLoad, ir.OpStore, ir.OpAddrOf, ir.OpFieldLoad, ir.OpFieldStore, ir.OpIndexLoad, ir.OpIndexStore, ir.OpGlobalLoad, ir.OpGlobalStore, ir.OpMakeClosure, ir.OpCall, ir.OpExceptionValue, ir.OpUnsupported}
	classes   = []ir.TypeClass{ir.ClassPointer, ir.ClassInterface, ir.ClassFunc, ir.ClassContainer, ir.ClassStruct, ir.ClassBool, ir.ClassString, ir.ClassNumber, ir.ClassOther}
	termKinds = []ir.TermKind{ir.TermJump, ir.TermBranch, ir.TermReturn, ir.TermThrow, ir.TermUnreachable}
	edgeKinds = []ir.EdgeKind{ir.EdgeNormal, ir.EdgeTrue, ir.EdgeFalse, ir.EdgeException, ir.EdgeDefer, ir.EdgeBack}
	constKind = []ir.ConstKind{ir.ConstString, ir.ConstInt, ir.ConstFloat, ir.ConstBool, ir.ConstNil}
	calleeKnd = []ir.CalleeKind{ir.CalleeStatic, ir.CalleeDynamic, ir.CalleeValue}
	stepKinds = []ir.StepKind{ir.StepField, ir.StepIndex, ir.StepDeref}
)

const escape = 255

func enumCode[T ~string](list []T, v T) (byte, bool) {
	if v == "" {
		return 0, true
	}
	for i, x := range list {
		if x == v {
			return byte(i + 1), true
		}
	}
	return escape, false
}

// encoder holds the state of one record.
type encoder struct {
	strs    map[string]uint64
	list    []string
	body    []byte
	lastOff int
	lastLn  int
	lastF   string
}

// Encode appends the record of f to dst.
func Encode(dst []byte, f *ir.Func) []byte {
	e := &encoder{strs: map[string]uint64{}}
	e.body = make([]byte, 0, 256+64*len(f.Blocks))
	e.fn(f)
	start := len(dst)
	dst = append(dst, make([]byte, headerLen)...)
	dst = binary.AppendUvarint(dst, uint64(len(e.list)))
	for _, s := range e.list {
		dst = binary.AppendUvarint(dst, uint64(len(s)))
		dst = append(dst, s...)
	}
	dst = append(dst, e.body...)
	body := dst[start+headerLen:]
	binary.LittleEndian.PutUint32(dst[start:], uint32(len(body)))
	binary.LittleEndian.PutUint32(dst[start+4:], crc32.Checksum(body, crcTable))
	return dst
}

func (e *encoder) u(v uint64) { e.body = binary.AppendUvarint(e.body, v) }
func (e *encoder) i(v int)    { e.body = binary.AppendVarint(e.body, int64(v)) }
func (e *encoder) b(v byte)   { e.body = append(e.body, v) }

// num is the number of a string: 0 is "", else its place in the list of the record (the first is 1).
func (e *encoder) num(v string) uint64 {
	if v == "" {
		return 0
	}
	n, ok := e.strs[v]
	if !ok {
		e.list = append(e.list, v)
		n = uint64(len(e.list))
		e.strs[v] = n
	}
	return n
}

// s writes a string as its number.
func (e *encoder) s(v string) { e.u(e.num(v)) }

func encEnum[T ~string](e *encoder, list []T, v T) {
	c, ok := enumCode(list, v)
	e.b(c)
	if !ok {
		e.s(string(v))
	}
}

func (e *encoder) pos(p ir.Pos) {
	switch {
	case p.File == "":
		e.u(0)
	case p.File == e.lastF:
		e.u(1)
	default:
		e.u(e.num(p.File) + 1)
	}
	if p.File != "" {
		e.lastF = p.File
	}
	e.i(p.Offset - e.lastOff)
	e.i(p.Line - e.lastLn)
	e.i(p.Length)
	e.i(p.Col)
	e.lastOff, e.lastLn = p.Offset, p.Line
}

func (e *encoder) ids(v []ir.LocalID) {
	e.u(uint64(len(v)))
	for _, x := range v {
		e.i(int(x))
	}
}

func (e *encoder) fn(f *ir.Func) {
	var fl uint64
	for i, set := range []bool{f.Exported, f.Entry, f.Init, f.Recv, f.Dep, f.Private, f.ExternalEntry, !f.Pos.IsZero()} {
		if set {
			fl |= 1 << i
		}
	}
	e.u(fl)
	e.s(string(f.ID))
	e.s(f.File)
	e.s(f.Symbol)
	if fl&(1<<7) != 0 {
		e.pos(f.Pos)
	}
	e.u(uint64(len(f.Overrides)))
	for _, o := range f.Overrides {
		e.s(o)
	}
	e.u(uint64(len(f.Params)))
	for _, p := range f.Params {
		e.s(p.Name)
		e.s(string(p.Type))
		encEnum(e, classes, p.Class)
		e.s(p.Display)
		if p.Captured {
			e.b(1)
		} else {
			e.b(0)
		}
	}
	e.u(uint64(len(f.Results)))
	for _, r := range f.Results {
		e.s(string(r))
	}
	e.u(uint64(len(f.Locals)))
	for _, l := range f.Locals {
		e.s(l.Name)
		e.s(string(l.Type))
		encEnum(e, classes, l.Class)
	}
	e.u(uint64(len(f.Roots)))
	for _, r := range f.Roots {
		e.i(int(r))
	}
	e.u(uint64(len(f.Blocks)))
	for bi := range f.Blocks {
		b := &f.Blocks[bi]
		e.i(int(b.ID) - bi)
		e.u(uint64(len(b.Instrs)))
		for ii := range b.Instrs {
			e.instr(&b.Instrs[ii])
		}
		e.term(&b.Term)
		e.u(uint64(len(b.Succs)))
		for _, s := range b.Succs {
			e.i(int(s.To))
			encEnum(e, edgeKinds, s.Kind)
		}
	}
}

func (e *encoder) term(t *ir.Term) {
	var m uint64
	for i, set := range []bool{t.Cond != 0, !t.CondPos.IsZero(), len(t.Values) > 0, !t.Pos.IsZero()} {
		if set {
			m |= 1 << i
		}
	}
	encEnum(e, termKinds, t.Kind)
	e.u(m)
	if m&1 != 0 {
		e.i(int(t.Cond))
	}
	if m&2 != 0 {
		e.pos(t.CondPos)
	}
	if m&4 != 0 {
		e.ids(t.Values)
	}
	if m&8 != 0 {
		e.pos(t.Pos)
	}
}

func (e *encoder) instr(in *ir.Instr) {
	var m uint32
	set := func(bit uint32, on bool) {
		if on {
			m |= bit
		}
	}
	set(mDsts, len(in.Dsts) > 0)
	set(mArgs, len(in.Args) > 0)
	set(mPos, !in.Pos.IsZero())
	set(mType, in.Type != "")
	set(mField, in.Field != "")
	set(mConst, in.Const != nil)
	set(mCallee, in.Callee != nil)
	set(mOperator, in.Operator != "")
	set(mIndex, in.Index != 0)
	set(mClass, in.Class != "")
	set(mPath, len(in.Path) > 0)
	set(mGlobal, in.Global != "")
	set(mFunc, in.Func != "")
	set(mView, in.View)
	set(mFolded, in.Folded)
	set(mKeyed, in.Keyed)
	set(mCharRead, in.CharRead)
	set(mGo, in.Go)
	set(mDefer, in.Defer)
	set(mAwait, in.Await)
	set(mReason, in.Reason != "")
	set(mAddrPos, !in.AddrPos.IsZero())
	set(mArgPos, len(in.ArgPos) > 0)
	set(mArgNames, len(in.ArgNames) > 0)
	encEnum(e, ops, in.Op)
	e.u(uint64(m))
	if m&mDsts != 0 {
		e.ids(in.Dsts)
	}
	if m&mArgs != 0 {
		e.ids(in.Args)
	}
	if m&mPos != 0 {
		e.pos(in.Pos)
	}
	if m&mType != 0 {
		e.s(string(in.Type))
	}
	if m&mField != 0 {
		e.s(in.Field)
	}
	if m&mConst != 0 {
		encEnum(e, constKind, in.Const.Kind)
		if in.Const.Truncated {
			e.b(1)
		} else {
			e.b(0)
		}
		e.s(in.Const.Value)
	}
	if m&mCallee != 0 {
		e.callee(in.Callee)
	}
	if m&mOperator != 0 {
		e.s(in.Operator)
	}
	if m&mIndex != 0 {
		e.i(in.Index)
	}
	if m&mClass != 0 {
		encEnum(e, classes, in.Class)
	}
	if m&mPath != 0 {
		e.u(uint64(len(in.Path)))
		for _, s := range in.Path {
			encEnum(e, stepKinds, s.Kind)
			e.s(s.Field)
		}
	}
	if m&mGlobal != 0 {
		e.s(in.Global)
	}
	if m&mFunc != 0 {
		e.s(string(in.Func))
	}
	if m&mReason != 0 {
		e.s(in.Reason)
	}
	if m&mAddrPos != 0 {
		e.pos(in.AddrPos)
	}
	if m&mArgPos != 0 {
		e.u(uint64(len(in.ArgPos)))
		for _, p := range in.ArgPos {
			e.pos(p)
		}
	}
	if m&mArgNames != 0 {
		e.u(uint64(len(in.ArgNames)))
		for _, n := range in.ArgNames {
			e.s(n)
		}
	}
}

func (e *encoder) callee(c *ir.Callee) {
	var m uint64
	for i, set := range []bool{c.Func != "", c.Symbol != "", c.Recv, len(c.Candidates) > 0, c.FFI} {
		if set {
			m |= 1 << i
		}
	}
	encEnum(e, calleeKnd, c.Kind)
	e.u(m)
	if m&1 != 0 {
		e.s(string(c.Func))
	}
	if m&2 != 0 {
		e.s(c.Symbol)
	}
	if m&8 != 0 {
		e.u(uint64(len(c.Candidates)))
		for _, x := range c.Candidates {
			e.s(string(x))
		}
	}
}

// decoder reads one body. The first error stops it: every read after it gives zero.
type decoder struct {
	b               []byte
	strs            []string
	err             error
	lastOff, lastLn int
	lastF           string
}

func (d *decoder) fail(what string) {
	if d.err == nil {
		d.err = fmt.Errorf("%w: %s", ErrCorrupt, what)
	}
	d.b = nil
}

func (d *decoder) u() uint64 {
	v, n := binary.Uvarint(d.b)
	if n <= 0 {
		d.fail("number")
		return 0
	}
	d.b = d.b[n:]
	return v
}

// i reads a signed number that holds a value (a position: the extractor of Rust writes the offset as line * 100000 +
// column, so it passes 2^32 in a file of 43000 lines). All 64 bits are kept; nothing is checked, because no bound is
// known for a value.
func (d *decoder) i() int {
	v, n := binary.Varint(d.b)
	if n <= 0 {
		d.fail("number")
		return 0
	}
	d.b = d.b[n:]
	return int(v)
}

// idx reads a signed number that is the place of a local, a block or a parameter: one that does not fit what the
// writer made (more than 4 * MaxInt32) is corrupt.
func (d *decoder) idx() int { return d.place(d.i()) }

// place checks a number that is a place.
func (d *decoder) place(v int) int {
	if d.err == nil && (v > math.MaxInt32*4 || v < -math.MaxInt32*4) {
		d.fail("number out of range")
		return 0
	}
	return v
}

func (d *decoder) byte() byte {
	if len(d.b) == 0 {
		d.fail("short")
		return 0
	}
	v := d.b[0]
	d.b = d.b[1:]
	return v
}

// count reads a count of things that take at least one byte each: more than the bytes left is corrupt.
func (d *decoder) count() int {
	n := d.u()
	if n > uint64(len(d.b)) {
		d.fail("count")
		return 0
	}
	return int(n)
}

func (d *decoder) str() string {
	n := d.u()
	return d.strN(n)
}

func (d *decoder) strN(n uint64) string {
	if n == 0 {
		return ""
	}
	if n > uint64(len(d.strs)) {
		d.fail("string number")
		return ""
	}
	return d.strs[n-1]
}

func decEnum[T ~string](d *decoder, list []T) T {
	c := d.byte()
	switch {
	case c == 0:
		return ""
	case c == escape:
		return T(d.str())
	case int(c) <= len(list):
		return list[c-1]
	}
	d.fail("enum")
	return ""
}

func (d *decoder) pos() ir.Pos {
	var p ir.Pos
	switch n := d.u(); n {
	case 0:
	case 1:
		p.File = d.lastF
	default:
		p.File = d.strN(n - 1)
	}
	if p.File != "" {
		d.lastF = p.File
	}
	p.Offset = d.lastOff + d.i()
	p.Line = d.lastLn + d.i()
	p.Length = d.i()
	p.Col = d.i()
	d.lastOff, d.lastLn = p.Offset, p.Line
	return p
}

func (d *decoder) ids() []ir.LocalID {
	n := d.count()
	if n == 0 {
		return nil
	}
	v := make([]ir.LocalID, n)
	for i := range v {
		v[i] = ir.LocalID(d.idx())
	}
	return v
}

// Decode reads the record at the start of b, and says how many bytes it took. It checks the length (against
// MaxRecord and what b holds) and the CRC before it reads anything else.
func Decode(b []byte) (*ir.Func, int, error) {
	if len(b) < headerLen {
		return nil, 0, fmt.Errorf("%w: short header", ErrCorrupt)
	}
	n := binary.LittleEndian.Uint32(b)
	if n > MaxRecord || uint64(n) > uint64(len(b)-headerLen) {
		return nil, 0, fmt.Errorf("%w: length %d", ErrCorrupt, n)
	}
	body := b[headerLen : headerLen+int(n)]
	if crc32.Checksum(body, crcTable) != binary.LittleEndian.Uint32(b[4:]) {
		return nil, 0, fmt.Errorf("%w: CRC", ErrCorrupt)
	}
	f, err := decodeBody(body)
	if err != nil {
		return nil, 0, err
	}
	return f, headerLen + int(n), nil
}

func decodeBody(body []byte) (*ir.Func, error) {
	d := &decoder{b: body}
	ns := d.count()
	d.strs = make([]string, 0, ns)
	for i := 0; i < ns && d.err == nil; i++ {
		l := d.u()
		if l > uint64(len(d.b)) {
			d.fail("string length")
			break
		}
		d.strs = append(d.strs, string(d.b[:l]))
		d.b = d.b[l:]
	}
	f := d.fn()
	if d.err == nil && len(d.b) != 0 {
		d.fail("trailing bytes")
	}
	if d.err != nil {
		return nil, d.err
	}
	return f, nil
}

func (d *decoder) fn() *ir.Func {
	f := &ir.Func{}
	fl := d.u()
	f.Exported, f.Entry, f.Init, f.Recv = fl&1 != 0, fl&2 != 0, fl&4 != 0, fl&8 != 0
	f.Dep, f.Private, f.ExternalEntry = fl&16 != 0, fl&32 != 0, fl&64 != 0
	f.ID = ir.FuncID(d.str())
	f.File = d.str()
	f.Symbol = d.str()
	if fl&(1<<7) != 0 {
		f.Pos = d.pos()
	}
	if n := d.count(); n > 0 {
		f.Overrides = make([]string, n)
		for i := range f.Overrides {
			f.Overrides[i] = d.str()
		}
	}
	if n := d.count(); n > 0 {
		f.Params = make([]ir.Param, n)
		for i := range f.Params {
			p := &f.Params[i]
			p.Name = d.str()
			p.Type = ir.TypeID(d.str())
			p.Class = decEnum[ir.TypeClass](d, classes)
			p.Display = d.str()
			p.Captured = d.byte() != 0
		}
	}
	if n := d.count(); n > 0 {
		f.Results = make([]ir.TypeID, n)
		for i := range f.Results {
			f.Results[i] = ir.TypeID(d.str())
		}
	}
	if n := d.count(); n > 0 {
		f.Locals = make([]ir.Local, n)
		for i := range f.Locals {
			l := &f.Locals[i]
			l.Name = d.str()
			l.Type = ir.TypeID(d.str())
			l.Class = decEnum[ir.TypeClass](d, classes)
		}
	}
	if n := d.count(); n > 0 {
		f.Roots = make([]ir.BlockID, n)
		for i := range f.Roots {
			f.Roots[i] = ir.BlockID(d.idx())
		}
	}
	if n := d.count(); n > 0 {
		f.Blocks = make([]ir.Block, n)
		for bi := range f.Blocks {
			b := &f.Blocks[bi]
			b.ID = ir.BlockID(d.place(bi + d.i()))
			if k := d.count(); k > 0 {
				b.Instrs = make([]ir.Instr, k)
				for ii := range b.Instrs {
					d.instr(&b.Instrs[ii])
				}
			}
			d.term(&b.Term)
			if k := d.count(); k > 0 {
				b.Succs = make([]ir.Edge, k)
				for si := range b.Succs {
					b.Succs[si].To = ir.BlockID(d.idx())
					b.Succs[si].Kind = decEnum[ir.EdgeKind](d, edgeKinds)
				}
			}
		}
	}
	return f
}

func (d *decoder) term(t *ir.Term) {
	t.Kind = decEnum[ir.TermKind](d, termKinds)
	m := d.u()
	if m&1 != 0 {
		t.Cond = ir.LocalID(d.idx())
	}
	if m&2 != 0 {
		t.CondPos = d.pos()
	}
	if m&4 != 0 {
		t.Values = d.ids()
	}
	if m&8 != 0 {
		t.Pos = d.pos()
	}
}

func (d *decoder) instr(in *ir.Instr) {
	in.Op = decEnum[ir.Op](d, ops)
	mm := d.u()
	if mm >= uint64(mMax) {
		d.fail("mask")
		return
	}
	m := uint32(mm)
	if m&mDsts != 0 {
		in.Dsts = d.ids()
	}
	if m&mArgs != 0 {
		in.Args = d.ids()
	}
	if m&mPos != 0 {
		in.Pos = d.pos()
	}
	if m&mType != 0 {
		in.Type = ir.TypeID(d.str())
	}
	if m&mField != 0 {
		in.Field = d.str()
	}
	if m&mConst != 0 {
		c := &ir.Const{}
		c.Kind = decEnum[ir.ConstKind](d, constKind)
		c.Truncated = d.byte() != 0
		c.Value = d.str()
		in.Const = c
	}
	if m&mCallee != 0 {
		in.Callee = d.callee()
	}
	if m&mOperator != 0 {
		in.Operator = d.str()
	}
	if m&mIndex != 0 {
		in.Index = d.idx()
	}
	if m&mClass != 0 {
		in.Class = decEnum[ir.TypeClass](d, classes)
	}
	if m&mPath != 0 {
		in.Path = make([]ir.Step, d.count())
		for i := range in.Path {
			in.Path[i].Kind = decEnum[ir.StepKind](d, stepKinds)
			in.Path[i].Field = d.str()
		}
	}
	if m&mGlobal != 0 {
		in.Global = d.str()
	}
	if m&mFunc != 0 {
		in.Func = ir.FuncID(d.str())
	}
	in.View, in.Folded, in.Keyed, in.CharRead = m&mView != 0, m&mFolded != 0, m&mKeyed != 0, m&mCharRead != 0
	in.Go, in.Defer, in.Await = m&mGo != 0, m&mDefer != 0, m&mAwait != 0
	if m&mReason != 0 {
		in.Reason = d.str()
	}
	if m&mAddrPos != 0 {
		in.AddrPos = d.pos()
	}
	if m&mArgPos != 0 {
		in.ArgPos = make([]ir.Pos, d.count())
		for i := range in.ArgPos {
			in.ArgPos[i] = d.pos()
		}
	}
	if m&mArgNames != 0 {
		in.ArgNames = make([]string, d.count())
		for i := range in.ArgNames {
			in.ArgNames[i] = d.str()
		}
	}
}

func (d *decoder) callee() *ir.Callee {
	c := &ir.Callee{}
	c.Kind = decEnum[ir.CalleeKind](d, calleeKnd)
	m := d.u()
	if m&1 != 0 {
		c.Func = ir.FuncID(d.str())
	}
	if m&2 != 0 {
		c.Symbol = d.str()
	}
	c.Recv = m&4 != 0
	if m&8 != 0 {
		c.Candidates = make([]ir.FuncID, d.count())
		for i := range c.Candidates {
			c.Candidates[i] = ir.FuncID(d.str())
		}
	}
	c.FFI = m&16 != 0
	return c
}
