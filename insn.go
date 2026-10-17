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
	Units   uint32      // width in code units (payloads can exceed 65535 units)
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
	SwitchTargets  []int32 // relative offsets (code units) from the switch instruction
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

// ArgRegs returns the argument registers for both list and range invoke forms.
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

// insnShape classifies the instruction at code-unit offset off and computes
// its width without decoding operands. It performs exactly the validation
// that DecodeInsn applies to an instruction's boundaries, so SweepInsns and
// DecodeInsn agree on every boundary and every error. For payloads, size is
// the entry count (switches) or the element byte length (fill-array-data).
//
// A 0x00 opcode whose high byte is not a payload identifier is a plain nop
// of one code unit, as the Android runtime computes it.
func insnShape(insns []byte, off uint32) (b []byte, op uint8, kind PayloadKind, units uint32, size uint64, err error) {
	pos := int(off) * 2
	if pos < 0 || pos+2 > len(insns) {
		return nil, 0, PayloadNone, 0, 0, fmt.Errorf("instruction offset 0x%x out of bounds", off)
	}
	b = insns[pos:]
	op = b[0]
	if op == 0x00 && b[1] != 0 {
		avail := uint64(len(b))
		switch le16(b) {
		case PayloadPackedSwitch:
			if avail < 8 {
				return nil, 0, PayloadNone, 0, 0, fmt.Errorf("packed-switch-payload at 0x%x truncated", off)
			}
			n := uint64(le16(b[2:]))
			u := 4 + n*2
			if u*2 > avail {
				return nil, 0, PayloadNone, 0, 0, fmt.Errorf("packed-switch-payload at 0x%x: %d targets exceed code", off, n)
			}
			return b, 0, PayloadPacked, uint32(u), n, nil
		case PayloadSparseSwitch:
			if avail < 4 {
				return nil, 0, PayloadNone, 0, 0, fmt.Errorf("sparse-switch-payload at 0x%x truncated", off)
			}
			n := uint64(le16(b[2:]))
			u := 2 + n*4
			if u*2 > avail {
				return nil, 0, PayloadNone, 0, 0, fmt.Errorf("sparse-switch-payload at 0x%x: %d entries exceed code", off, n)
			}
			return b, 0, PayloadSparse, uint32(u), n, nil
		case PayloadFillArrayData:
			if avail < 8 {
				return nil, 0, PayloadNone, 0, 0, fmt.Errorf("fill-array-data-payload at 0x%x truncated", off)
			}
			dataLen := uint64(le16(b[2:])) * uint64(le32(b[4:]))
			u := 4 + (dataLen+1)/2
			if u*2 > avail {
				return nil, 0, PayloadNone, 0, 0, fmt.Errorf("fill-array-data-payload at 0x%x: %d bytes exceed code", off, dataLen)
			}
			return b, 0, PayloadFill, uint32(u), dataLen, nil
		}
		// Any other high byte: a one-unit nop.
	}
	info := &Opcodes[op]
	units = uint32(info.Format.Units())
	if pos+int(units)*2 > len(insns) {
		return nil, 0, PayloadNone, 0, 0, fmt.Errorf("instruction %s at 0x%x truncated (needs %d code units)", info.Name, off, units)
	}
	if (info.Format == Fmt35c || info.Format == Fmt45cc) && b[1]>>4 > 5 {
		return nil, 0, PayloadNone, 0, 0, fmt.Errorf("%s at 0x%x: invalid register count %d", info.Name, off, b[1]>>4)
	}
	return b, op, PayloadNone, units, 0, nil
}

// DecodeInsn decodes the instruction at code-unit offset off within insns.
// insns is the raw little-endian instruction array (byte length is even).
// Payloads are recognised by their pseudo-opcode and decoded fully.
func DecodeInsn(insns []byte, off uint32) (Insn, error) {
	b, op, kind, units, size, err := insnShape(insns, off)
	if err != nil {
		return Insn{}, err
	}
	in := Insn{Off: off, Units: units, Op: op, Payload: kind}
	if kind != PayloadNone {
		in.Fmt = Fmt10x
		switch kind {
		case PayloadPacked:
			n := int(size)
			in.PackedFirstKey = int32(le32(b[4:]))
			in.SwitchTargets = make([]int32, n)
			for i := 0; i < n; i++ {
				in.SwitchTargets[i] = int32(le32(b[8+i*4:]))
			}
		case PayloadSparse:
			n := int(size)
			in.SwitchKeys = make([]int32, n)
			in.SwitchTargets = make([]int32, n)
			for i := 0; i < n; i++ {
				in.SwitchKeys[i] = int32(le32(b[4+i*4:]))
				in.SwitchTargets[i] = int32(le32(b[4+n*4+i*4:]))
			}
		case PayloadFill:
			in.FillWidth = le16(b[2:])
			in.FillCount = le32(b[4:])
			in.FillData = b[8 : 8+size]
		}
		return in, nil
	}

	info := &Opcodes[op]
	in.Fmt = info.Format
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
		// A|G|op BBBB F|E|D|C [HHHH]; insnShape has validated A <= 5.
		count := uint32(b[1] >> 4)
		in.A = count
		in.Idx = uint32(le16(b[2:]))
		regs := []uint16{uint16(b[4] & 0x0f), uint16(b[4] >> 4), uint16(b[5] & 0x0f), uint16(b[5] >> 4), uint16(b[1] & 0x0f)}
		in.Regs = regs[:count]
		if info.Format == Fmt45cc {
			in.Idx2 = uint32(le16(b[6:]))
		}
	case Fmt3rc, Fmt4rcc:
		// AA|op BBBB CCCC [HHHH]
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

// InsnRec is the compact per-instruction record kept for a decoded method:
// enough to locate and classify every instruction without retaining operands.
type InsnRec struct {
	Off     uint32
	Units   uint32
	Op      uint8
	Payload PayloadKind
}

// SweepInsns performs a linear sweep over the instruction array, returning
// one record per instruction or payload. Decoding stops at the first
// undecodable position; the returned error describes it while the records
// decoded so far are still returned. Only instruction boundaries are
// computed; operands are not decoded.
func SweepInsns(insns []byte) ([]InsnRec, error) {
	n := uint32(len(insns) / 2)
	recs := make([]InsnRec, 0, n/2+1)
	var off uint32
	for off < n {
		_, op, kind, units, _, err := insnShape(insns, off)
		if err != nil {
			return recs, err
		}
		recs = append(recs, InsnRec{Off: off, Units: units, Op: op, Payload: kind})
		off += units
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
		} else if off >= r.Off+r.Units {
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
