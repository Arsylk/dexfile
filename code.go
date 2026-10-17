package dexfile

import (
	"fmt"
)

// TryItem is a try_item with its range.
type TryItem struct {
	StartAddr  uint32 // code units
	InsnCount  uint16 // code units
	HandlerOff uint16 // byte offset from start of encoded_catch_handler_list
	Range      Range
	HandlerIdx int32 // index into CodeItem.Handlers, -1 if unresolved
}

// EndAddr returns the exclusive end of the try range in code units.
func (t TryItem) EndAddr() uint32 { return t.StartAddr + uint32(t.InsnCount) }

// TypeAddrPair is one typed catch clause.
type TypeAddrPair struct {
	TypeIdx uint32
	Addr    uint32 // handler code unit offset
}

// CatchHandler is an encoded_catch_handler.
type CatchHandler struct {
	ListOff      uint16 // byte offset from start of the handler list (matches try_item.handler_off)
	Range        Range
	Catches      []TypeAddrPair
	CatchAllAddr int64 // -1 if none
}

// CodeItem is a parsed code_item.
type CodeItem struct {
	Off           uint32
	RegistersSize uint16
	InsSize       uint16
	OutsSize      uint16
	TriesSize     uint16
	DebugInfoOff  uint32
	InsnsSize     uint32 // code units

	Range         Range // whole item including tries and handlers
	InsnsRange    Range // raw instruction bytes
	TriesRange    Range // try_items (zero if none)
	HandlersRange Range // encoded_catch_handler_list (zero if none)

	Tries    []TryItem
	Handlers []CatchHandler

	insnsErr error // first instruction decoding problem, if any (lazy)
	recs     []InsnRec
	recsDone bool
}

// Insns returns the raw instruction bytes (little-endian code units).
func (ci *CodeItem) Insns(f *File) []byte {
	b, _ := f.Bytes(ci.InsnsRange)
	return b
}

// Code parses (and caches) the code_item at the given offset.
func (f *File) Code(off uint32) (*CodeItem, error) {
	f.codeMu.Lock()
	if ci, ok := f.codeCache[off]; ok {
		f.codeMu.Unlock()
		return ci, nil
	}
	f.codeMu.Unlock()
	ci, err := f.parseCode(off, nil)
	if err != nil {
		return nil, err
	}
	f.codeMu.Lock()
	if existing, ok := f.codeCache[off]; ok {
		ci = existing
	} else {
		f.codeCache[off] = ci
	}
	f.codeMu.Unlock()
	return ci, nil
}

// CodeFields re-parses a code_item recording field views (header, tries,
// handlers). Instruction bytes are recorded as one opaque field.
func (f *File) CodeFields(off uint32) ([]FieldView, *CodeItem, error) {
	var views []FieldView
	ci, err := f.parseCode(off, &views)
	return views, ci, err
}

func (f *File) parseCode(off uint32, rec *[]FieldView) (*CodeItem, error) {
	if off%4 != 0 {
		return nil, perr("code_item", off, ErrBadAlign)
	}
	c := newCursor(f.data, off, "code_item")
	c.rec = rec
	ci := &CodeItem{Off: off}
	ci.RegistersSize = c.u16("registers_size")
	ci.InsSize = c.u16("ins_size")
	ci.OutsSize = c.u16("outs_size")
	ci.TriesSize = c.u16("tries_size")
	ci.DebugInfoOff = c.u32("debug_info_off")
	ci.InsnsSize = c.u32("insns_size")
	if c.Err() != nil {
		return nil, c.Err()
	}
	if ci.InsSize > ci.RegistersSize {
		f.warn(fmt.Sprintf("code_item ins_size %d > registers_size %d", ci.InsSize, ci.RegistersSize), off)
	}
	insnsBytes := uint64(ci.InsnsSize) * 2
	if insnsBytes > uint64(c.remaining()) {
		return nil, perr("code_item.insns", off+16, fmt.Errorf("%w: insns_size %d code units", ErrTooLarge, ci.InsnsSize))
	}
	ci.InsnsRange = Range{off + 16, uint32(insnsBytes)}
	if rec != nil {
		c.record("insns", int(off+16), int(insnsBytes), fmt.Sprintf("<%d code units>", ci.InsnsSize))
	}
	c.skip(int(insnsBytes))
	if ci.TriesSize > 0 {
		if ci.InsnsSize%2 == 1 {
			c.u16("padding")
		}
		triesStart := c.Pos()
		if uint64(ci.TriesSize)*8 > uint64(c.remaining()) {
			return nil, perr("code_item.tries", triesStart, ErrTooLarge)
		}
		ci.Tries = make([]TryItem, ci.TriesSize)
		for i := range ci.Tries {
			t := &ci.Tries[i]
			start := c.Pos()
			t.StartAddr = c.u32(fmt.Sprintf("tries[%d].start_addr", i))
			t.InsnCount = c.u16(fmt.Sprintf("tries[%d].insn_count", i))
			t.HandlerOff = c.u16(fmt.Sprintf("tries[%d].handler_off", i))
			t.Range = Range{start, 8}
			t.HandlerIdx = -1
			if c.Err() == nil && uint64(t.StartAddr)+uint64(t.InsnCount) > uint64(ci.InsnsSize) {
				f.warn(fmt.Sprintf("try_item %d range [%d,+%d) exceeds insns_size %d", i, t.StartAddr, t.InsnCount, ci.InsnsSize), start)
			}
		}
		if c.Err() != nil {
			return nil, c.Err()
		}
		ci.TriesRange = Range{triesStart, uint32(ci.TriesSize) * 8}
		listStart := c.Pos()
		n := c.uleb("handlers.size")
		if c.Err() != nil {
			return nil, c.Err()
		}
		if uint64(n) > uint64(c.remaining()) {
			return nil, perr("encoded_catch_handler_list", listStart, ErrTooLarge)
		}
		ci.Handlers = make([]CatchHandler, 0, n)
		for i := uint32(0); i < n; i++ {
			hStart := c.Pos()
			rel := hStart - listStart
			if rel > 0xffff {
				return nil, perr("encoded_catch_handler", hStart, fmt.Errorf("%w: handler offset exceeds 16 bits", ErrInvalid))
			}
			size := c.sleb(fmt.Sprintf("handlers[%d].size", i))
			if c.Err() != nil {
				return nil, c.Err()
			}
			abs := size
			if abs < 0 {
				abs = -abs
			}
			if uint64(abs)*2 > uint64(c.remaining()) {
				return nil, perr("encoded_catch_handler", hStart, ErrTooLarge)
			}
			h := CatchHandler{ListOff: uint16(rel), CatchAllAddr: -1, Catches: make([]TypeAddrPair, 0, abs)}
			for j := int32(0); j < abs; j++ {
				ti := c.uleb(fmt.Sprintf("handlers[%d].handlers[%d].type_idx", i, j))
				addr := c.uleb(fmt.Sprintf("handlers[%d].handlers[%d].addr", i, j))
				if c.Err() != nil {
					return nil, c.Err()
				}
				if ti >= f.Header.TypeIDsSize {
					return nil, perr("encoded_catch_handler", hStart, fmt.Errorf("%w: type_idx %d", ErrInvalid, ti))
				}
				if addr >= ci.InsnsSize {
					f.warn(fmt.Sprintf("catch handler address %d outside insns", addr), hStart)
				}
				h.Catches = append(h.Catches, TypeAddrPair{TypeIdx: ti, Addr: addr})
			}
			if size <= 0 {
				ca := c.uleb(fmt.Sprintf("handlers[%d].catch_all_addr", i))
				if c.Err() != nil {
					return nil, c.Err()
				}
				h.CatchAllAddr = int64(ca)
				if ca >= ci.InsnsSize {
					f.warn(fmt.Sprintf("catch-all address %d outside insns", ca), hStart)
				}
			}
			h.Range = Range{hStart, c.Pos() - hStart}
			ci.Handlers = append(ci.Handlers, h)
		}
		ci.HandlersRange = Range{listStart, c.Pos() - listStart}
		// Link tries to handlers by byte offset (not list index).
		for i := range ci.Tries {
			t := &ci.Tries[i]
			for j := range ci.Handlers {
				if ci.Handlers[j].ListOff == t.HandlerOff {
					t.HandlerIdx = int32(j)
					break
				}
			}
			if t.HandlerIdx < 0 {
				f.warn(fmt.Sprintf("try_item %d handler_off %d does not start a handler", i, t.HandlerOff), t.Range.Off)
			}
		}
	}
	ci.Range = Range{off, c.Pos() - off}
	if c.Err() != nil {
		return nil, c.Err()
	}
	return ci, nil
}

// InsnRecs returns the linear-sweep instruction records for the code item,
// decoding them on first use. The error (if any) reports where decoding
// stopped; records before that point remain valid.
func (ci *CodeItem) InsnRecs(f *File) ([]InsnRec, error) {
	f.codeMu.Lock()
	defer f.codeMu.Unlock()
	if !ci.recsDone {
		ci.recs, ci.insnsErr = SweepInsns(ci.Insns(f))
		ci.recsDone = true
	}
	return ci.recs, ci.insnsErr
}

// Decode decodes the instruction at a code-unit offset within this code item.
func (ci *CodeItem) Decode(f *File, off uint32) (Insn, error) {
	return DecodeInsn(ci.Insns(f), off)
}

// InsnFileOffset converts a code-unit offset to an absolute file offset.
func (ci *CodeItem) InsnFileOffset(off uint32) uint32 { return ci.InsnsRange.Off + off*2 }

// HandlersFor returns the catch handler covering a code-unit address, choosing
// the innermost (first matching in table order, as the VM does).
func (ci *CodeItem) HandlersFor(addr uint32) *CatchHandler {
	for i := range ci.Tries {
		t := &ci.Tries[i]
		if addr >= t.StartAddr && addr < t.EndAddr() && t.HandlerIdx >= 0 {
			return &ci.Handlers[t.HandlerIdx]
		}
	}
	return nil
}

// EncodedMethodByIdx finds the encoded_method for a method index within class
// data, returning the method and whether it is virtual.
func (cd *ClassData) FindMethod(methodIdx uint32) (*EncodedMethod, bool, bool) {
	for i := range cd.DirectMethods {
		if cd.DirectMethods[i].MethodIdx == methodIdx {
			return &cd.DirectMethods[i], false, true
		}
	}
	for i := range cd.VirtualMethods {
		if cd.VirtualMethods[i].MethodIdx == methodIdx {
			return &cd.VirtualMethods[i], true, true
		}
	}
	return nil, false, false
}

// FindField finds the encoded_field for a field index within class data.
func (cd *ClassData) FindField(fieldIdx uint32) (*EncodedField, bool, bool) {
	for i := range cd.StaticFields {
		if cd.StaticFields[i].FieldIdx == fieldIdx {
			return &cd.StaticFields[i], true, true
		}
	}
	for i := range cd.InstanceFields {
		if cd.InstanceFields[i].FieldIdx == fieldIdx {
			return &cd.InstanceFields[i], false, true
		}
	}
	return nil, false, false
}
