package dexfile

// Format identifies the encoding of a Dalvik instruction.
type Format uint8

const (
	Fmt10x Format = iota
	Fmt12x
	Fmt11n
	Fmt11x
	Fmt10t
	Fmt20t
	Fmt22x
	Fmt21t
	Fmt21s
	Fmt21h
	Fmt21c
	Fmt23x
	Fmt22b
	Fmt22t
	Fmt22s
	Fmt22c
	Fmt32x
	Fmt30t
	Fmt31t
	Fmt31i
	Fmt31c
	Fmt35c
	Fmt3rc
	Fmt45cc
	Fmt4rcc
	Fmt51l
	FmtUnused // reserved opcode; decoded as a single code unit
)

var formatNames = [...]string{"10x", "12x", "11n", "11x", "10t", "20t", "22x", "21t", "21s", "21h", "21c", "23x", "22b", "22t", "22s", "22c", "32x", "30t", "31t", "31i", "31c", "35c", "3rc", "45cc", "4rcc", "51l", "unused"}

func (f Format) String() string {
	if int(f) < len(formatNames) {
		return formatNames[f]
	}
	return "?"
}

// Units returns the instruction width in 16-bit code units.
func (f Format) Units() uint16 {
	switch f {
	case Fmt10x, Fmt12x, Fmt11n, Fmt11x, Fmt10t, FmtUnused:
		return 1
	case Fmt20t, Fmt22x, Fmt21t, Fmt21s, Fmt21h, Fmt21c, Fmt23x, Fmt22b, Fmt22t, Fmt22s, Fmt22c:
		return 2
	case Fmt32x, Fmt30t, Fmt31t, Fmt31i, Fmt31c, Fmt35c, Fmt3rc:
		return 3
	case Fmt45cc, Fmt4rcc:
		return 4
	case Fmt51l:
		return 5
	}
	return 1
}

// RefKind identifies what table an instruction's index operand refers to.
type RefKind uint8

const (
	RefNone RefKind = iota
	RefString
	RefType
	RefField
	RefMethod
	RefProto
	RefCallSite
	RefMethodHandle
)

var refKindNames = [...]string{"none", "string", "type", "field", "method", "proto", "call_site", "method_handle"}

func (k RefKind) String() string {
	if int(k) < len(refKindNames) {
		return refKindNames[k]
	}
	return "?"
}

// OpFlags carry semantic properties of an opcode.
type OpFlags uint32

const (
	FlagContinue      OpFlags = 1 << iota // execution may fall through to the next instruction
	FlagBranch                            // conditional or unconditional branch (has a target)
	FlagSwitch                            // packed/sparse switch
	FlagReturn                            // returns from the method
	FlagThrow                             // throw instruction
	FlagCanThrow                          // may raise an exception (invoke, field access, etc.)
	FlagInvoke                            // invoke-* instruction
	FlagSetsResult                        // leaves a value in the "result" pseudo-register
	FlagFieldGet                          // instance/static field read
	FlagFieldPut                          // instance/static field write
	FlagArrayGet                          // array element read
	FlagArrayPut                          // array element write
	FlagWideDest                          // destination register is a wide pair
	FlagObjectDest                        // destination holds an object reference
	FlagPayload                           // pseudo-instruction payload (not executable)
	FlagMoveResult                        // move-result*/move-exception
	FlagUnary                             // unary arithmetic / conversion
	FlagBinary                            // binary arithmetic (23x / 12x 2addr)
	FlagBinaryLit                         // binary arithmetic with literal (22b / 22s)
	FlagUnconditional                     // goto family
	FlagCompare                           // cmp* instruction
	FlagMove                              // register move
	FlagConst                             // loads a constant
	FlagStatic                            // static field op or invoke-static
)

// OpInfo describes one opcode.
type OpInfo struct {
	Name   string
	Format Format
	Ref    RefKind
	Flags  OpFlags
}

const c = FlagContinue

// Opcodes is the complete Dalvik opcode table indexed by opcode byte.
var Opcodes = [256]OpInfo{
	0x00: {"nop", Fmt10x, RefNone, c},
	0x01: {"move", Fmt12x, RefNone, c | FlagMove},
	0x02: {"move/from16", Fmt22x, RefNone, c | FlagMove},
	0x03: {"move/16", Fmt32x, RefNone, c | FlagMove},
	0x04: {"move-wide", Fmt12x, RefNone, c | FlagMove | FlagWideDest},
	0x05: {"move-wide/from16", Fmt22x, RefNone, c | FlagMove | FlagWideDest},
	0x06: {"move-wide/16", Fmt32x, RefNone, c | FlagMove | FlagWideDest},
	0x07: {"move-object", Fmt12x, RefNone, c | FlagMove | FlagObjectDest},
	0x08: {"move-object/from16", Fmt22x, RefNone, c | FlagMove | FlagObjectDest},
	0x09: {"move-object/16", Fmt32x, RefNone, c | FlagMove | FlagObjectDest},
	0x0a: {"move-result", Fmt11x, RefNone, c | FlagMoveResult},
	0x0b: {"move-result-wide", Fmt11x, RefNone, c | FlagMoveResult | FlagWideDest},
	0x0c: {"move-result-object", Fmt11x, RefNone, c | FlagMoveResult | FlagObjectDest},
	0x0d: {"move-exception", Fmt11x, RefNone, c | FlagMoveResult | FlagObjectDest},
	0x0e: {"return-void", Fmt10x, RefNone, FlagReturn},
	0x0f: {"return", Fmt11x, RefNone, FlagReturn},
	0x10: {"return-wide", Fmt11x, RefNone, FlagReturn},
	0x11: {"return-object", Fmt11x, RefNone, FlagReturn},
	0x12: {"const/4", Fmt11n, RefNone, c | FlagConst},
	0x13: {"const/16", Fmt21s, RefNone, c | FlagConst},
	0x14: {"const", Fmt31i, RefNone, c | FlagConst},
	0x15: {"const/high16", Fmt21h, RefNone, c | FlagConst},
	0x16: {"const-wide/16", Fmt21s, RefNone, c | FlagConst | FlagWideDest},
	0x17: {"const-wide/32", Fmt31i, RefNone, c | FlagConst | FlagWideDest},
	0x18: {"const-wide", Fmt51l, RefNone, c | FlagConst | FlagWideDest},
	0x19: {"const-wide/high16", Fmt21h, RefNone, c | FlagConst | FlagWideDest},
	0x1a: {"const-string", Fmt21c, RefString, c | FlagConst | FlagObjectDest | FlagCanThrow},
	0x1b: {"const-string/jumbo", Fmt31c, RefString, c | FlagConst | FlagObjectDest | FlagCanThrow},
	0x1c: {"const-class", Fmt21c, RefType, c | FlagConst | FlagObjectDest | FlagCanThrow},
	0x1d: {"monitor-enter", Fmt11x, RefNone, c | FlagCanThrow},
	0x1e: {"monitor-exit", Fmt11x, RefNone, c | FlagCanThrow},
	0x1f: {"check-cast", Fmt21c, RefType, c | FlagCanThrow | FlagObjectDest},
	0x20: {"instance-of", Fmt22c, RefType, c | FlagCanThrow},
	0x21: {"array-length", Fmt12x, RefNone, c | FlagCanThrow},
	0x22: {"new-instance", Fmt21c, RefType, c | FlagCanThrow | FlagObjectDest},
	0x23: {"new-array", Fmt22c, RefType, c | FlagCanThrow | FlagObjectDest},
	0x24: {"filled-new-array", Fmt35c, RefType, c | FlagCanThrow | FlagSetsResult},
	0x25: {"filled-new-array/range", Fmt3rc, RefType, c | FlagCanThrow | FlagSetsResult},
	0x26: {"fill-array-data", Fmt31t, RefNone, c | FlagCanThrow},
	0x27: {"throw", Fmt11x, RefNone, FlagThrow | FlagCanThrow},
	0x28: {"goto", Fmt10t, RefNone, FlagBranch | FlagUnconditional},
	0x29: {"goto/16", Fmt20t, RefNone, FlagBranch | FlagUnconditional},
	0x2a: {"goto/32", Fmt30t, RefNone, FlagBranch | FlagUnconditional},
	0x2b: {"packed-switch", Fmt31t, RefNone, c | FlagSwitch},
	0x2c: {"sparse-switch", Fmt31t, RefNone, c | FlagSwitch},
	0x2d: {"cmpl-float", Fmt23x, RefNone, c | FlagCompare},
	0x2e: {"cmpg-float", Fmt23x, RefNone, c | FlagCompare},
	0x2f: {"cmpl-double", Fmt23x, RefNone, c | FlagCompare},
	0x30: {"cmpg-double", Fmt23x, RefNone, c | FlagCompare},
	0x31: {"cmp-long", Fmt23x, RefNone, c | FlagCompare},
	0x32: {"if-eq", Fmt22t, RefNone, c | FlagBranch},
	0x33: {"if-ne", Fmt22t, RefNone, c | FlagBranch},
	0x34: {"if-lt", Fmt22t, RefNone, c | FlagBranch},
	0x35: {"if-ge", Fmt22t, RefNone, c | FlagBranch},
	0x36: {"if-gt", Fmt22t, RefNone, c | FlagBranch},
	0x37: {"if-le", Fmt22t, RefNone, c | FlagBranch},
	0x38: {"if-eqz", Fmt21t, RefNone, c | FlagBranch},
	0x39: {"if-nez", Fmt21t, RefNone, c | FlagBranch},
	0x3a: {"if-ltz", Fmt21t, RefNone, c | FlagBranch},
	0x3b: {"if-gez", Fmt21t, RefNone, c | FlagBranch},
	0x3c: {"if-gtz", Fmt21t, RefNone, c | FlagBranch},
	0x3d: {"if-lez", Fmt21t, RefNone, c | FlagBranch},
	0x3e: {"unused-3e", FmtUnused, RefNone, 0},
	0x3f: {"unused-3f", FmtUnused, RefNone, 0},
	0x40: {"unused-40", FmtUnused, RefNone, 0},
	0x41: {"unused-41", FmtUnused, RefNone, 0},
	0x42: {"unused-42", FmtUnused, RefNone, 0},
	0x43: {"unused-43", FmtUnused, RefNone, 0},
	0x44: {"aget", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayGet},
	0x45: {"aget-wide", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayGet | FlagWideDest},
	0x46: {"aget-object", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayGet | FlagObjectDest},
	0x47: {"aget-boolean", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayGet},
	0x48: {"aget-byte", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayGet},
	0x49: {"aget-char", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayGet},
	0x4a: {"aget-short", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayGet},
	0x4b: {"aput", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayPut},
	0x4c: {"aput-wide", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayPut},
	0x4d: {"aput-object", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayPut},
	0x4e: {"aput-boolean", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayPut},
	0x4f: {"aput-byte", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayPut},
	0x50: {"aput-char", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayPut},
	0x51: {"aput-short", Fmt23x, RefNone, c | FlagCanThrow | FlagArrayPut},
	0x52: {"iget", Fmt22c, RefField, c | FlagCanThrow | FlagFieldGet},
	0x53: {"iget-wide", Fmt22c, RefField, c | FlagCanThrow | FlagFieldGet | FlagWideDest},
	0x54: {"iget-object", Fmt22c, RefField, c | FlagCanThrow | FlagFieldGet | FlagObjectDest},
	0x55: {"iget-boolean", Fmt22c, RefField, c | FlagCanThrow | FlagFieldGet},
	0x56: {"iget-byte", Fmt22c, RefField, c | FlagCanThrow | FlagFieldGet},
	0x57: {"iget-char", Fmt22c, RefField, c | FlagCanThrow | FlagFieldGet},
	0x58: {"iget-short", Fmt22c, RefField, c | FlagCanThrow | FlagFieldGet},
	0x59: {"iput", Fmt22c, RefField, c | FlagCanThrow | FlagFieldPut},
	0x5a: {"iput-wide", Fmt22c, RefField, c | FlagCanThrow | FlagFieldPut},
	0x5b: {"iput-object", Fmt22c, RefField, c | FlagCanThrow | FlagFieldPut},
	0x5c: {"iput-boolean", Fmt22c, RefField, c | FlagCanThrow | FlagFieldPut},
	0x5d: {"iput-byte", Fmt22c, RefField, c | FlagCanThrow | FlagFieldPut},
	0x5e: {"iput-char", Fmt22c, RefField, c | FlagCanThrow | FlagFieldPut},
	0x5f: {"iput-short", Fmt22c, RefField, c | FlagCanThrow | FlagFieldPut},
	0x60: {"sget", Fmt21c, RefField, c | FlagCanThrow | FlagFieldGet | FlagStatic},
	0x61: {"sget-wide", Fmt21c, RefField, c | FlagCanThrow | FlagFieldGet | FlagStatic | FlagWideDest},
	0x62: {"sget-object", Fmt21c, RefField, c | FlagCanThrow | FlagFieldGet | FlagStatic | FlagObjectDest},
	0x63: {"sget-boolean", Fmt21c, RefField, c | FlagCanThrow | FlagFieldGet | FlagStatic},
	0x64: {"sget-byte", Fmt21c, RefField, c | FlagCanThrow | FlagFieldGet | FlagStatic},
	0x65: {"sget-char", Fmt21c, RefField, c | FlagCanThrow | FlagFieldGet | FlagStatic},
	0x66: {"sget-short", Fmt21c, RefField, c | FlagCanThrow | FlagFieldGet | FlagStatic},
	0x67: {"sput", Fmt21c, RefField, c | FlagCanThrow | FlagFieldPut | FlagStatic},
	0x68: {"sput-wide", Fmt21c, RefField, c | FlagCanThrow | FlagFieldPut | FlagStatic},
	0x69: {"sput-object", Fmt21c, RefField, c | FlagCanThrow | FlagFieldPut | FlagStatic},
	0x6a: {"sput-boolean", Fmt21c, RefField, c | FlagCanThrow | FlagFieldPut | FlagStatic},
	0x6b: {"sput-byte", Fmt21c, RefField, c | FlagCanThrow | FlagFieldPut | FlagStatic},
	0x6c: {"sput-char", Fmt21c, RefField, c | FlagCanThrow | FlagFieldPut | FlagStatic},
	0x6d: {"sput-short", Fmt21c, RefField, c | FlagCanThrow | FlagFieldPut | FlagStatic},
	0x6e: {"invoke-virtual", Fmt35c, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0x6f: {"invoke-super", Fmt35c, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0x70: {"invoke-direct", Fmt35c, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0x71: {"invoke-static", Fmt35c, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult | FlagStatic},
	0x72: {"invoke-interface", Fmt35c, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0x73: {"unused-73", FmtUnused, RefNone, 0},
	0x74: {"invoke-virtual/range", Fmt3rc, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0x75: {"invoke-super/range", Fmt3rc, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0x76: {"invoke-direct/range", Fmt3rc, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0x77: {"invoke-static/range", Fmt3rc, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult | FlagStatic},
	0x78: {"invoke-interface/range", Fmt3rc, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0x79: {"unused-79", FmtUnused, RefNone, 0},
	0x7a: {"unused-7a", FmtUnused, RefNone, 0},
	0x7b: {"neg-int", Fmt12x, RefNone, c | FlagUnary},
	0x7c: {"not-int", Fmt12x, RefNone, c | FlagUnary},
	0x7d: {"neg-long", Fmt12x, RefNone, c | FlagUnary | FlagWideDest},
	0x7e: {"not-long", Fmt12x, RefNone, c | FlagUnary | FlagWideDest},
	0x7f: {"neg-float", Fmt12x, RefNone, c | FlagUnary},
	0x80: {"neg-double", Fmt12x, RefNone, c | FlagUnary | FlagWideDest},
	0x81: {"int-to-long", Fmt12x, RefNone, c | FlagUnary | FlagWideDest},
	0x82: {"int-to-float", Fmt12x, RefNone, c | FlagUnary},
	0x83: {"int-to-double", Fmt12x, RefNone, c | FlagUnary | FlagWideDest},
	0x84: {"long-to-int", Fmt12x, RefNone, c | FlagUnary},
	0x85: {"long-to-float", Fmt12x, RefNone, c | FlagUnary},
	0x86: {"long-to-double", Fmt12x, RefNone, c | FlagUnary | FlagWideDest},
	0x87: {"float-to-int", Fmt12x, RefNone, c | FlagUnary},
	0x88: {"float-to-long", Fmt12x, RefNone, c | FlagUnary | FlagWideDest},
	0x89: {"float-to-double", Fmt12x, RefNone, c | FlagUnary | FlagWideDest},
	0x8a: {"double-to-int", Fmt12x, RefNone, c | FlagUnary},
	0x8b: {"double-to-long", Fmt12x, RefNone, c | FlagUnary | FlagWideDest},
	0x8c: {"double-to-float", Fmt12x, RefNone, c | FlagUnary},
	0x8d: {"int-to-byte", Fmt12x, RefNone, c | FlagUnary},
	0x8e: {"int-to-char", Fmt12x, RefNone, c | FlagUnary},
	0x8f: {"int-to-short", Fmt12x, RefNone, c | FlagUnary},
	0x90: {"add-int", Fmt23x, RefNone, c | FlagBinary},
	0x91: {"sub-int", Fmt23x, RefNone, c | FlagBinary},
	0x92: {"mul-int", Fmt23x, RefNone, c | FlagBinary},
	0x93: {"div-int", Fmt23x, RefNone, c | FlagBinary | FlagCanThrow},
	0x94: {"rem-int", Fmt23x, RefNone, c | FlagBinary | FlagCanThrow},
	0x95: {"and-int", Fmt23x, RefNone, c | FlagBinary},
	0x96: {"or-int", Fmt23x, RefNone, c | FlagBinary},
	0x97: {"xor-int", Fmt23x, RefNone, c | FlagBinary},
	0x98: {"shl-int", Fmt23x, RefNone, c | FlagBinary},
	0x99: {"shr-int", Fmt23x, RefNone, c | FlagBinary},
	0x9a: {"ushr-int", Fmt23x, RefNone, c | FlagBinary},
	0x9b: {"add-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0x9c: {"sub-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0x9d: {"mul-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0x9e: {"div-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest | FlagCanThrow},
	0x9f: {"rem-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest | FlagCanThrow},
	0xa0: {"and-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xa1: {"or-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xa2: {"xor-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xa3: {"shl-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xa4: {"shr-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xa5: {"ushr-long", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xa6: {"add-float", Fmt23x, RefNone, c | FlagBinary},
	0xa7: {"sub-float", Fmt23x, RefNone, c | FlagBinary},
	0xa8: {"mul-float", Fmt23x, RefNone, c | FlagBinary},
	0xa9: {"div-float", Fmt23x, RefNone, c | FlagBinary},
	0xaa: {"rem-float", Fmt23x, RefNone, c | FlagBinary},
	0xab: {"add-double", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xac: {"sub-double", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xad: {"mul-double", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xae: {"div-double", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xaf: {"rem-double", Fmt23x, RefNone, c | FlagBinary | FlagWideDest},
	0xb0: {"add-int/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xb1: {"sub-int/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xb2: {"mul-int/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xb3: {"div-int/2addr", Fmt12x, RefNone, c | FlagBinary | FlagCanThrow},
	0xb4: {"rem-int/2addr", Fmt12x, RefNone, c | FlagBinary | FlagCanThrow},
	0xb5: {"and-int/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xb6: {"or-int/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xb7: {"xor-int/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xb8: {"shl-int/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xb9: {"shr-int/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xba: {"ushr-int/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xbb: {"add-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xbc: {"sub-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xbd: {"mul-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xbe: {"div-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest | FlagCanThrow},
	0xbf: {"rem-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest | FlagCanThrow},
	0xc0: {"and-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xc1: {"or-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xc2: {"xor-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xc3: {"shl-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xc4: {"shr-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xc5: {"ushr-long/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xc6: {"add-float/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xc7: {"sub-float/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xc8: {"mul-float/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xc9: {"div-float/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xca: {"rem-float/2addr", Fmt12x, RefNone, c | FlagBinary},
	0xcb: {"add-double/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xcc: {"sub-double/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xcd: {"mul-double/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xce: {"div-double/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xcf: {"rem-double/2addr", Fmt12x, RefNone, c | FlagBinary | FlagWideDest},
	0xd0: {"add-int/lit16", Fmt22s, RefNone, c | FlagBinaryLit},
	0xd1: {"rsub-int", Fmt22s, RefNone, c | FlagBinaryLit},
	0xd2: {"mul-int/lit16", Fmt22s, RefNone, c | FlagBinaryLit},
	0xd3: {"div-int/lit16", Fmt22s, RefNone, c | FlagBinaryLit | FlagCanThrow},
	0xd4: {"rem-int/lit16", Fmt22s, RefNone, c | FlagBinaryLit | FlagCanThrow},
	0xd5: {"and-int/lit16", Fmt22s, RefNone, c | FlagBinaryLit},
	0xd6: {"or-int/lit16", Fmt22s, RefNone, c | FlagBinaryLit},
	0xd7: {"xor-int/lit16", Fmt22s, RefNone, c | FlagBinaryLit},
	0xd8: {"add-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit},
	0xd9: {"rsub-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit},
	0xda: {"mul-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit},
	0xdb: {"div-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit | FlagCanThrow},
	0xdc: {"rem-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit | FlagCanThrow},
	0xdd: {"and-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit},
	0xde: {"or-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit},
	0xdf: {"xor-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit},
	0xe0: {"shl-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit},
	0xe1: {"shr-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit},
	0xe2: {"ushr-int/lit8", Fmt22b, RefNone, c | FlagBinaryLit},
	0xe3: {"unused-e3", FmtUnused, RefNone, 0},
	0xe4: {"unused-e4", FmtUnused, RefNone, 0},
	0xe5: {"unused-e5", FmtUnused, RefNone, 0},
	0xe6: {"unused-e6", FmtUnused, RefNone, 0},
	0xe7: {"unused-e7", FmtUnused, RefNone, 0},
	0xe8: {"unused-e8", FmtUnused, RefNone, 0},
	0xe9: {"unused-e9", FmtUnused, RefNone, 0},
	0xea: {"unused-ea", FmtUnused, RefNone, 0},
	0xeb: {"unused-eb", FmtUnused, RefNone, 0},
	0xec: {"unused-ec", FmtUnused, RefNone, 0},
	0xed: {"unused-ed", FmtUnused, RefNone, 0},
	0xee: {"unused-ee", FmtUnused, RefNone, 0},
	0xef: {"unused-ef", FmtUnused, RefNone, 0},
	0xf0: {"unused-f0", FmtUnused, RefNone, 0},
	0xf1: {"unused-f1", FmtUnused, RefNone, 0},
	0xf2: {"unused-f2", FmtUnused, RefNone, 0},
	0xf3: {"unused-f3", FmtUnused, RefNone, 0},
	0xf4: {"unused-f4", FmtUnused, RefNone, 0},
	0xf5: {"unused-f5", FmtUnused, RefNone, 0},
	0xf6: {"unused-f6", FmtUnused, RefNone, 0},
	0xf7: {"unused-f7", FmtUnused, RefNone, 0},
	0xf8: {"unused-f8", FmtUnused, RefNone, 0},
	0xf9: {"unused-f9", FmtUnused, RefNone, 0},
	0xfa: {"invoke-polymorphic", Fmt45cc, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0xfb: {"invoke-polymorphic/range", Fmt4rcc, RefMethod, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0xfc: {"invoke-custom", Fmt35c, RefCallSite, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0xfd: {"invoke-custom/range", Fmt3rc, RefCallSite, c | FlagCanThrow | FlagInvoke | FlagSetsResult},
	0xfe: {"const-method-handle", Fmt21c, RefMethodHandle, c | FlagConst | FlagObjectDest | FlagCanThrow},
	0xff: {"const-method-type", Fmt21c, RefProto, c | FlagConst | FlagObjectDest | FlagCanThrow},
}

// Payload pseudo-opcode identifiers (the full 16-bit first code unit).
const (
	PayloadPackedSwitch  uint16 = 0x0100
	PayloadSparseSwitch  uint16 = 0x0200
	PayloadFillArrayData uint16 = 0x0300
)

// MinVersionForOpcode returns the first DEX version in which the opcode is defined.
func MinVersionForOpcode(op uint8) int {
	switch {
	case op >= 0xfa && op <= 0xfd:
		return 38
	case op == 0xfe || op == 0xff:
		return 39
	}
	return 35
}
