package dexfile

import "fmt"

// Debug info state-machine opcodes.
const (
	DbgEndSequence        = 0x00
	DbgAdvancePC          = 0x01
	DbgAdvanceLine        = 0x02
	DbgStartLocal         = 0x03
	DbgStartLocalExtended = 0x04
	DbgEndLocal           = 0x05
	DbgRestartLocal       = 0x06
	DbgSetPrologueEnd     = 0x07
	DbgSetEpilogueBegin   = 0x08
	DbgSetFile            = 0x09
	DbgFirstSpecial       = 0x0a
	dbgLineBase           = -4
	dbgLineRange          = 15
)

// DebugOp is one decoded debug_info_item bytecode with its range.
type DebugOp struct {
	Opcode uint8   `json:"opcode"`
	Name   string  `json:"name"`
	Args   []int64 `json:"args,omitempty"`
	Range  Range   `json:"range"`
	// Resulting state after the op (for special opcodes and advance ops).
	Addr uint32 `json:"addr"`
	Line int32  `json:"line"`
}

// Position is one address/line mapping emitted by a special opcode.
type Position struct {
	Addr          uint32 `json:"addr"`
	Line          int32  `json:"line"`
	PrologueEnd   bool   `json:"prologue_end,omitempty"`
	EpilogueBegin bool   `json:"epilogue_begin,omitempty"`
	SourceFile    int32  `json:"source_file_idx"` // -1 if default
}

// Local is a local variable's live range as described by debug info.
type Local struct {
	Register  uint32 `json:"register"`
	NameIdx   int32  `json:"name_idx"` // -1 if absent
	TypeIdx   int32  `json:"type_idx"`
	SigIdx    int32  `json:"signature_idx"`
	StartAddr uint32 `json:"start_addr"`
	EndAddr   uint32 `json:"end_addr"` // exclusive; InsnsSize if still live at end
	Ended     bool   `json:"ended"`
	IsParam   bool   `json:"is_param,omitempty"` // synthesised from the method signature
	// SynthName is set for synthesised locals whose name is not in the string
	// table (currently only "this").
	SynthName string `json:"synth_name,omitempty"`
}

// DebugInfo is a parsed debug_info_item.
type DebugInfo struct {
	Range          Range
	LineStart      uint32
	ParameterNames []int32 // string idx or -1 per parameter
	Ops            []DebugOp
	Positions      []Position
	Locals         []Local
	Truncated      bool // decoding stopped early (op limit or error)
	Err            error
}

var dbgNames = [...]string{"DBG_END_SEQUENCE", "DBG_ADVANCE_PC", "DBG_ADVANCE_LINE", "DBG_START_LOCAL", "DBG_START_LOCAL_EXTENDED", "DBG_END_LOCAL", "DBG_RESTART_LOCAL", "DBG_SET_PROLOGUE_END", "DBG_SET_EPILOGUE_BEGIN", "DBG_SET_FILE"}

// maxDebugOps bounds the state machine so hostile input cannot spin.
const maxDebugOps = 1 << 20

// DebugInfo parses the debug_info_item at off. insnsSize (code units) is
// used to close still-open locals; pass 0 if unknown. The parser is lenient:
// a malformed tail yields a partially populated result with Err set.
// Parameter locals are not synthesised; use DebugInfoFor for that.
func (f *File) DebugInfo(off uint32, insnsSize uint32) (*DebugInfo, error) {
	return f.parseDebugInfo(off, insnsSize, nil, nil)
}

// paramSeed describes the method's parameters so that the debug state machine
// can treat them as locals live from address 0 (as the Android runtime and
// dexdump do).
type paramSeed struct {
	firstReg uint32
	static   bool
	class    uint32   // type idx of the declaring class (for "this")
	types    []uint16 // parameter type indexes
}

// DebugInfoFor parses the debug info of a method's code item, seeding
// parameter locals ("this" and named parameters) like the runtime does.
func (f *File) DebugInfoFor(code *CodeItem, methodIdx uint32, static bool) (*DebugInfo, error) {
	if code.DebugInfoOff == 0 {
		return nil, nil
	}
	seed := f.seedFor(code, methodIdx, static)
	return f.parseDebugInfo(code.DebugInfoOff, code.InsnsSize, nil, seed)
}

func (f *File) seedFor(code *CodeItem, methodIdx uint32, static bool) *paramSeed {
	if int(methodIdx) >= len(f.MethodIDs) {
		return nil
	}
	mid := f.MethodIDs[methodIdx]
	seed := &paramSeed{static: static, class: uint32(mid.ClassIdx)}
	if int(mid.ProtoIdx) < len(f.ProtoIDs) {
		if tl, err := f.TypeList(f.ProtoIDs[mid.ProtoIdx].ParametersOff); err == nil {
			seed.types = tl.Types
		}
	}
	if code.RegistersSize >= code.InsSize {
		seed.firstReg = uint32(code.RegistersSize - code.InsSize)
	}
	return seed
}

// DebugInfoFields re-parses a debug_info_item recording field views.
func (f *File) DebugInfoFields(off uint32, insnsSize uint32) ([]FieldView, *DebugInfo, error) {
	var views []FieldView
	di, err := f.parseDebugInfo(off, insnsSize, &views, nil)
	return views, di, err
}

func (f *File) parseDebugInfo(off uint32, insnsSize uint32, rec *[]FieldView, seed *paramSeed) (*DebugInfo, error) {
	c := newCursor(f.data, off, "debug_info_item")
	c.rec = rec
	di := &DebugInfo{}
	di.LineStart = c.uleb("line_start")
	n := c.uleb("parameters_size")
	if c.Err() != nil {
		return nil, c.Err()
	}
	if uint64(n) > uint64(c.remaining()) {
		return nil, perr("debug_info_item", off, fmt.Errorf("%w: parameters_size %d", ErrTooLarge, n))
	}
	di.ParameterNames = make([]int32, n)
	for i := range di.ParameterNames {
		var name string
		if rec != nil {
			name = fmt.Sprintf("parameter_names[%d]", i)
		}
		v := c.ulebp1(name)
		if v >= 0 && uint32(v) >= f.Header.StringIDsSize {
			f.warn("debug parameter name index out of range", c.Pos())
			v = -1
		}
		di.ParameterNames[i] = v
	}
	if c.Err() != nil {
		return nil, c.Err()
	}

	// State machine.
	addr := uint32(0)
	line := int32(di.LineStart)
	srcFile := int32(-1)
	prologueEnd, epilogueBegin := false, false
	type openLocal struct {
		idx int
	}
	live := map[uint32]int{} // register -> index in di.Locals of the open local
	if seed != nil {
		reg := seed.firstReg
		if !seed.static {
			di.Locals = append(di.Locals, Local{Register: reg, NameIdx: -1, TypeIdx: int32(seed.class), SigIdx: -1, IsParam: true, SynthName: "this"})
			live[reg] = len(di.Locals) - 1
			reg++
		}
		for i, t := range seed.types {
			name := int32(-1)
			if i < len(di.ParameterNames) {
				name = di.ParameterNames[i]
			}
			di.Locals = append(di.Locals, Local{Register: reg, NameIdx: name, TypeIdx: int32(t), SigIdx: -1, IsParam: true})
			live[reg] = len(di.Locals) - 1
			desc := f.TypeDesc(uint32(t))
			if desc == "J" || desc == "D" {
				reg += 2
			} else {
				reg++
			}
		}
	}
	closeLocal := func(reg uint32, at uint32) {
		if i, ok := live[reg]; ok {
			di.Locals[i].EndAddr = at
			di.Locals[i].Ended = true
			delete(live, reg)
		}
	}
	strIdx := func(v int32) int32 {
		if v >= 0 && uint32(v) >= f.Header.StringIDsSize {
			return -1
		}
		return v
	}
	typeIdx := func(v int32) int32 {
		if v >= 0 && uint32(v) >= f.Header.TypeIDsSize {
			return -1
		}
		return v
	}

	for count := 0; ; count++ {
		if count >= maxDebugOps {
			di.Truncated = true
			di.Err = perr("debug_info_item", c.Pos(), fmt.Errorf("%w: more than %d opcodes", ErrTooLarge, maxDebugOps))
			break
		}
		start := c.Pos()
		op := c.u8("")
		if c.Err() != nil {
			di.Truncated = true
			di.Err = c.Err()
			break
		}
		dop := DebugOp{Opcode: op}
		if op < DbgFirstSpecial {
			dop.Name = dbgNames[op]
		} else {
			dop.Name = "DBG_SPECIAL"
		}
		done := false
		switch op {
		case DbgEndSequence:
			done = true
		case DbgAdvancePC:
			d := c.uleb("addr_diff")
			addr += d
			dop.Args = []int64{int64(d)}
		case DbgAdvanceLine:
			d := c.sleb("line_diff")
			line += d
			dop.Args = []int64{int64(d)}
		case DbgStartLocal, DbgStartLocalExtended:
			reg := c.uleb("register_num")
			name := c.ulebp1("name_idx")
			typ := c.ulebp1("type_idx")
			sig := int32(-1)
			if op == DbgStartLocalExtended {
				sig = c.ulebp1("sig_idx")
			}
			dop.Args = []int64{int64(reg), int64(name), int64(typ)}
			if op == DbgStartLocalExtended {
				dop.Args = append(dop.Args, int64(sig))
			}
			if c.Err() == nil {
				closeLocal(reg, addr)
				di.Locals = append(di.Locals, Local{Register: reg, NameIdx: strIdx(name), TypeIdx: typeIdx(typ), SigIdx: strIdx(sig), StartAddr: addr})
				live[reg] = len(di.Locals) - 1
			}
		case DbgEndLocal:
			reg := c.uleb("register_num")
			dop.Args = []int64{int64(reg)}
			if c.Err() == nil {
				closeLocal(reg, addr)
			}
		case DbgRestartLocal:
			reg := c.uleb("register_num")
			dop.Args = []int64{int64(reg)}
			if c.Err() == nil {
				// Re-open the most recent local for this register.
				for i := len(di.Locals) - 1; i >= 0; i-- {
					if di.Locals[i].Register == reg {
						l := di.Locals[i]
						closeLocal(reg, addr)
						di.Locals = append(di.Locals, Local{Register: reg, NameIdx: l.NameIdx, TypeIdx: l.TypeIdx, SigIdx: l.SigIdx, StartAddr: addr, SynthName: l.SynthName})
						live[reg] = len(di.Locals) - 1
						break
					}
				}
			}
		case DbgSetPrologueEnd:
			prologueEnd = true
		case DbgSetEpilogueBegin:
			epilogueBegin = true
		case DbgSetFile:
			v := c.ulebp1("name_idx")
			srcFile = strIdx(v)
			dop.Args = []int64{int64(v)}
		default:
			adj := int32(op) - DbgFirstSpecial
			line += dbgLineBase + adj%dbgLineRange
			addr += uint32(adj / dbgLineRange)
			di.Positions = append(di.Positions, Position{Addr: addr, Line: line, PrologueEnd: prologueEnd, EpilogueBegin: epilogueBegin, SourceFile: srcFile})
			prologueEnd, epilogueBegin = false, false
		}
		if c.Err() != nil {
			di.Truncated = true
			di.Err = c.Err()
			break
		}
		dop.Addr = addr
		dop.Line = line
		dop.Range = Range{start, c.Pos() - start}
		if len(di.Ops) < 65536 {
			di.Ops = append(di.Ops, dop)
		}
		if done {
			break
		}
	}
	end := addr
	if insnsSize > end {
		end = insnsSize
	}
	for _, i := range live {
		di.Locals[i].EndAddr = end
	}
	// Drop zero-length ranges (a local re-declared at the same address, e.g. a
	// parameter immediately restarted with a generic signature).
	kept := di.Locals[:0]
	for _, l := range di.Locals {
		if l.EndAddr > l.StartAddr {
			kept = append(kept, l)
		}
	}
	di.Locals = kept
	di.Range = Range{off, c.Pos() - off}
	return di, nil
}

// LineForAddr returns the source line covering a code-unit address, using the
// last position at or before addr. ok is false if no position applies.
func (di *DebugInfo) LineForAddr(addr uint32) (line int32, ok bool) {
	for _, p := range di.Positions {
		if p.Addr <= addr {
			line, ok = p.Line, true
		} else {
			break
		}
	}
	return
}

// LocalsAt returns locals live at addr.
func (di *DebugInfo) LocalsAt(addr uint32) []Local {
	var out []Local
	for _, l := range di.Locals {
		if addr >= l.StartAddr && addr < l.EndAddr {
			out = append(out, l)
		}
	}
	return out
}
