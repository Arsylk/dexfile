package dexfile

import (
	"fmt"
)

// PayloadKind identifies a pseudo-instruction payload.
type PayloadKind uint8

const (
	PayloadNone PayloadKind = iota
	PayloadPacked
	PayloadSparse
	PayloadFill
)

func (p PayloadKind) String() string {
	switch p {
	case PayloadPacked:
		return "packed-switch-payload"
	case PayloadSparse:
		return "sparse-switch-payload"
	case PayloadFill:
		return "fill-array-data-payload"
	}
	return ""
}

// Insn is a fully decoded Dalvik instruction.
//
// Offsets are in 16-bit code units relative to the start of the method's
// instruction array. Register operands are stored in A, B, C according to
// the format's operand letters; the meaning of each is given by Format.
type Insn struct {
	Off     uint32      // code unit offset within insns
	Units   uint16      // width in code units
	Op      uint8       // opcode byte (0x00 for payloads)
	Fmt     Format      // encoding format
	Payload PayloadKind // non-zero for payload pseudo-instructions

	A, B, C uint32   // register (or count) operands per format
	Lit     int64    // literal operand (sign-extended, high16 shifted) if any
	Target  int32    // relative branch/payload offset in code units if any
	Idx     uint32   // primary table index (string/type/field/method/call_site/method_handle/proto)
	Idx2    uint32   // secondary index: proto for 45cc/4rcc
	Regs    []uint16 // explicit argument registers for 35c/45cc; nil for range forms

	// Payload contents (only for Payload != PayloadNone).
	PackedFirstKey int32
	SwitchKeys     []int32 // sparse: explicit keys; packed: nil (derive from first key)
	SwitchTargets  []int32 // relative offsets (code units) from the switch instruction... see note
	FillWidth      uint16
	FillCount      uint32
	FillData       []byte // raw element bytes (len = FillWidth*FillCount)
}

// Info returns the opcode table entry for this instruction.
func (in *Insn) Info() *OpInfo {
	if in.Payload != PayloadNone {
		return &payloadInfo
	}
	return &Opcodes[in.Op]
}

var payloadInfo = OpInfo{Name: "payload", Format: Fmt10x, Ref: RefNone, Flags: FlagPayload}

// Name returns the mnemonic.
func (in *Insn) Name() string {
	if in.Payload != PayloadNone {
		return in.Payload.String()
	}
	return Opcodes[in.Op].Name
}

// HasRef reports whether the instruction references a table entry.
func (in *Insn) HasRef() bool { return in.Payload == PayloadNone && Opcodes[in.Op].Ref != RefNone }

// RangeRegs returns the argument registers for both list and range invoke forms.
func (in *Insn) ArgRegs() []uint16 {
	if in.Regs != nil {
		return in.Regs
	}
	switch in.Fmt {
	case Fmt3rc, Fmt4rcc:
		n := int(in.A)
		out := make([]uint16, n)
		for i := 0; i < n; i++ {
			out[i] = uint16(in.C + uint32(i))
		}
		return out
	}
	return nil
}

// DecodeInsn decodes the instruction at code-unit offset off within insns.
// insns is the raw little-endian instruction array (byte length is even).
// Payloads are recognised by their pseudo-opcode and decoded fully.
func DecodeInsn(insns []byte, off uint32) (Insn, error) {
	pos := int(off) * 2
	if pos+2 > len(insns) || pos < 0 {
		return Insn{}, fmt.Errorf("instruction offset 0x%x out of bounds", off)
	}
	b := insns[pos:]
	op := b[0]
	in := Insn{Off: off, Op: op}

	if op == 0x00 && b[1] != 0 {
		return decodePayload(insns, off)
	}

	info := &Opcodes[op]
	in.Fmt = info.Format
	units := int(info.Format.Units())
	if pos+units*2 > len(insns) {
		return Insn{}, fmt.Errorf("instruction %s at 0x%x truncated (needs %d code units)", info.Name, off, units)
	}
	in.Units = uint16(units)

	switch info.Format {
	case Fmt10x, FmtUnused:
		// nothing
	case Fmt12x:
		in.A = uint32(b[1] & 0x0f)
		in.B = uint32(b[1] >> 4)
	case Fmt11n:
		in.A = uint32(b[1] & 0x0f)
		in.Lit = int64(int8(b[1]&0xf0) >> 4) // sign-extend nibble
	case Fmt11x:
		in.A = uint32(b[1])
	case Fmt10t:
		in.Target = int32(int8(b[1]))
	case Fmt20t:
		in.Target = int32(int16(le16(b[2:])))
	case Fmt22x:
		in.A = uint32(b[1])
		in.B = uint32(le16(b[2:]))
	case Fmt21t:
		in.A = uint32(b[1])
		in.Target = int32(int16(le16(b[2:])))
	case Fmt21s:
		in.A = uint32(b[1])
		in.Lit = int64(int16(le16(b[2:])))
	case Fmt21h:
		in.A = uint32(b[1])
		v := int64(int16(le16(b[2:])))
		if op == 0x19 { // const-wide/high16
			in.Lit = v << 48
		} else { // const/high16
			in.Lit = v << 16
		}
	case Fmt21c:
		in.A = uint32(b[1])
		in.Idx = uint32(le16(b[2:]))
	case Fmt23x:
		in.A = uint32(b[1])
		in.B = uint32(b[2])
		in.C = uint32(b[3])
	case Fmt22b:
		in.A = uint32(b[1])
		in.B = uint32(b[2])
		in.Lit = int64(int8(b[3]))
	case Fmt22t:
		in.A = uint32(b[1] & 0x0f)
		in.B = uint32(b[1] >> 4)
		in.Target = int32(int16(le16(b[2:])))
	case Fmt22s:
		in.A = uint32(b[1] & 0x0f)
		in.B = uint32(b[1] >> 4)
		in.Lit = int64(int16(le16(b[2:])))
	case Fmt22c:
		in.A = uint32(b[1] & 0x0f)
		in.B = uint32(b[1] >> 4)
		in.Idx = uint32(le16(b[2:]))
	case Fmt32x:
		in.A = uint32(le16(b[2:]))
		in.B = uint32(le16(b[4:]))
	case Fmt30t:
		in.Target = int32(le32(b[2:]))
	case Fmt31t:
		in.A = uint32(b[1])
		in.Target = int32(le32(b[2:]))
	case Fmt31i:
		in.A = uint32(b[1])
		in.Lit = int64(int32(le32(b[2:])))
	case Fmt31c:
		in.A = uint32(b[1])
		in.Idx = le32(b[2:])
	case Fmt35c, Fmt45cc:
		// A|G op BBBB F|E|D|C [HHHH]
		count := uint32(b[1] >> 4)
		g := uint16(b[1] & 0x0f)
		in.A = count
		in.Idx = uint32(le16(b[2:]))
		regs := []uint16{uint16(b[4] & 0x0f), uint16(b[4] >> 4), uint16(b[5] & 0x0f), uint16(b[5] >> 4), g}
		if count > 5 {
			return Insn{}, fmt.Errorf("%s at 0x%x: invalid register count %d", info.Name, off, count)
		}
		in.Regs = regs[:count]
		if info.Format == Fmt45cc {
			in.Idx2 = uint32(le16(b[6:]))
		}
	case Fmt3rc, Fmt4rcc:
		// AA op BBBB CCCC [HHHH]
		in.A = uint32(b[1])
		in.Idx = uint32(le16(b[2:]))
		in.C = uint32(le16(b[4:]))
		if info.Format == Fmt4rcc {
			in.Idx2 = uint32(le16(b[6:]))
		}
	case Fmt51l:
		in.A = uint32(b[1])
		in.Lit = int64(le64(b[2:]))
	default:
		return Insn{}, fmt.Errorf("unknown format for opcode 0x%02x", op)
	}
	return in, nil
}

func decodePayload(insns []byte, off uint32) (Insn, error) {
	pos := int(off) * 2
	b := insns[pos:]
	ident := le16(b)
	in := Insn{Off: off, Op: 0, Fmt: Fmt10x}
	avail := len(b)
	switch ident {
	case PayloadPackedSwitch:
		if avail < 8 {
			return Insn{}, fmt.Errorf("packed-switch-payload at 0x%x truncated", off)
		}
		size := int(le16(b[2:]))
		units := 4 + size*2
		if units*2 > avail {
			return Insn{}, fmt.Errorf("packed-switch-payload at 0x%x: %d targets exceed code", off, size)
		}
		in.Payload = PayloadPacked
		in.Units = uint16(units)
		in.PackedFirstKey = int32(le32(b[4:]))
		in.SwitchTargets = make([]int32, size)
		for i := 0; i < size; i++ {
			in.SwitchTargets[i] = int32(le32(b[8+i*4:]))
		}
	case PayloadSparseSwitch:
		if avail < 4 {
			return Insn{}, fmt.Errorf("sparse-switch-payload at 0x%x truncated", off)
		}
		size := int(le16(b[2:]))
		units := 2 + size*4
		if units*2 > avail {
			return Insn{}, fmt.Errorf("sparse-switch-payload at 0x%x: %d entries exceed code", off, size)
		}
		in.Payload = PayloadSparse
		in.Units = uint16(units)
		in.SwitchKeys = make([]int32, size)
		in.SwitchTargets = make([]int32, size)
		for i := 0; i < size; i++ {
			in.SwitchKeys[i] = int32(le32(b[4+i*4:]))
			in.SwitchTargets[i] = int32(le32(b[4+size*4+i*4:]))
		}
	case PayloadFillArrayData:
		if avail < 8 {
			return Insn{}, fmt.Errorf("fill-array-data-payload at 0x%x truncated", off)
		}
		width := le16(b[2:])
		count := le32(b[4:])
		dataLen := uint64(width) * uint64(count)
		units := 4 + (dataLen+1)/2
		if units*2 > uint64(avail) || units > 0xffff {
			return Insn{}, fmt.Errorf("fill-array-data-payload at 0x%x: %d bytes exceed code", off, dataLen)
		}
		in.Payload = PayloadFill
		in.Units = uint16(units)
		in.FillWidth = width
		in.FillCount = count
		in.FillData = b[8 : 8+dataLen]
	default:
		return Insn{}, fmt.Errorf("unknown payload ident 0x%04x at 0x%x", ident, off)
	}
	return in, nil
}

// InsnRec is the compact per-instruction record kept for a decoded method:
// enough to locate and classify every instruction without retaining operands.
type InsnRec struct {
	Off     uint32
	Units   uint16
	Op      uint8
	Payload PayloadKind
}

// SweepInsns performs a linear sweep over the instruction array, returning
// one record per instruction or payload. Decoding stops at the first
// undecodable position; the returned error describes it while the records
// decoded so far are still returned.
func SweepInsns(insns []byte) ([]InsnRec, error) {
	n := uint32(len(insns) / 2)
	recs := make([]InsnRec, 0, n/2+1)
	var off uint32
	for off < n {
		in, err := DecodeInsn(insns, off)
		if err != nil {
			return recs, err
		}
		recs = append(recs, InsnRec{Off: off, Units: in.Units, Op: in.Op, Payload: in.Payload})
		off += uint32(in.Units)
	}
	return recs, nil
}

// FindInsn locates the record containing the code-unit offset using binary search.
// Returns the index, or -1 if the offset is not within any instruction.
func FindInsn(recs []InsnRec, off uint32) int {
	lo, hi := 0, len(recs)
	for lo < hi {
		mid := (lo + hi) / 2
		r := recs[mid]
		if off < r.Off {
			hi = mid
		} else if off >= r.Off+uint32(r.Units) {
			lo = mid + 1
		} else {
			return mid
		}
	}
	return -1
}

// BranchTargets returns the absolute code-unit targets reachable from the
// instruction via branches or switches. For switches the payload must be
// supplied (it may be nil if unresolved, in which case only the fall-through
// is implied and no explicit targets are returned).
func (in *Insn) BranchTargets(payload *Insn) []uint32 {
	info := in.Info()
	switch {
	case info.Flags&FlagBranch != 0:
		return []uint32{uint32(int64(in.Off) + int64(in.Target))}
	case info.Flags&FlagSwitch != 0 && payload != nil:
		out := make([]uint32, 0, len(payload.SwitchTargets))
		for _, t := range payload.SwitchTargets {
			out = append(out, uint32(int64(in.Off)+int64(t)))
		}
		return out
	}
	return nil
}
