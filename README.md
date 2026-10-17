# dexfile

A bounds-safe, byte-range-aware parser and instruction decoder for Android
DEX files, written in pure Go with no dependencies outside the standard
library. It is the parser behind [dex-mcp](https://github.com/Arsylk/dex-mcp)
and is meant to be used directly by any tool that needs exact, verifiable
access to DEX structures: class tables, encoded members, code items with
tries and handlers, every instruction format and payload, encoded values and
annotations, debug info, call sites, method handles and hidden-API flags.

```
go get github.com/arsylk/dexfile
```

## Use

```go
import "github.com/arsylk/dexfile"

f, err := dexfile.Parse(data) // hostile input yields an error, never a panic
if err != nil {
	return err
}
for i, cd := range f.ClassDefs {
	fmt.Println(f.TypeDesc(cd.ClassIdx))
	cls := f.ClassData(uint32(i)) // nil when the class has no class_data_item
	if cls == nil {
		continue
	}
	for _, m := range append(cls.DirectMethods, cls.VirtualMethods...) {
		if m.CodeOff == 0 {
			continue // abstract or native
		}
		code, err := f.Code(m.CodeOff)
		if err != nil {
			return err
		}
		recs, err := code.InsnRecs(f) // one record per instruction or payload
		if err != nil {
			return err
		}
		insn, _ := code.Decode(f, recs[0].Off) // full operands for one instruction
		fmt.Printf("  %s: %d instructions, first %s\n", f.MethodDesc(m.MethodIdx), len(recs), insn.Name())
	}
}
```

`example_test.go` is the runnable form of this snippet. See `go doc
github.com/arsylk/dexfile` for the full API: `File` for the tables and
lookups, `CodeItem`/`Insn` for code, `EncodedValue`/`AnnotationItem` for
static values and annotations, `DebugInfo` for positions and locals, and
`Structures`/`StructuresAt` for the registry of byte ranges that maps any file
offset back to the structures containing it.

## Design

* Every parsed structure carries `Range{Off, Len}`, and the same parse code
  can record per-field ranges (`FieldView`) for hex inspection.
* All reads go through a bounds-checked cursor; LEB128 is limited to five
  bytes; counts are validated against the remaining bytes before allocation;
  nesting depth of encoded values is bounded; a `recover` guard at the parse
  entry point turns any residual panic into an error.
* The opcode table is complete (including 0xfa–0xff for DEX 038+), formats
  45cc/4rcc are decoded, payloads are validated, try/handler linking follows
  byte offsets, and float/double, method-type and method-handle encoded
  values are decoded as the specification requires.
* DEX versions 035 through 041 are supported, with best-effort handling of
  later versions. ODEX/VDEX/CDEX and big-endian files are rejected with a
  clear error.

## Verification

* Every fixture under `testdata/dex` is compared instruction by instruction
  against AOSP `dexdump` output (offsets, bytes, mnemonics, registers,
  literals, targets, reference indices), plus headers, class tables,
  try/catch tables, debug positions and locals, call sites and method
  handles, static values and annotations.
* Malformed inputs, exhaustive truncation and bit-flips must never panic
  (`TestTruncationNeverPanics`, `TestBitflipsNeverPanic`), and there are Go
  fuzz targets (`FuzzParse`, `FuzzDecodeInsn`).
* `internal/upstream` runs differential tests against
  `github.com/grassto/androguard-go` with every disagreement classified; see
  `docs/UPSTREAM_AUDIT.md`. That module is a test-only dependency.

```
go test ./...
go test . -fuzz FuzzParse -fuzztime 60s
```

## Fixtures

`testdata/dex` is generated from the Java sources under `testdata/java` by
`testdata/build.sh` (needs a JDK and Android build-tools `d8`/`dexdump`);
`testdata/README.md` describes every fixture. The committed files are the
ones the tests run against, so the build script is only needed to change
them.

## Non-goals

No decompilation, no APK modification, no execution of analysed code, no
ODEX/VDEX/CDEX parsing. Cross-reference indexes, control-flow graphs and
data-flow live in [dex-mcp](https://github.com/Arsylk/dex-mcp).
