# dexfile

A bounds-safe, byte-range-aware parser and instruction decoder for Android
DEX files, written in pure Go with no dependencies outside the standard
library. It is the parser behind [dex-mcp](https://github.com/Arsylk/dex-mcp)
and is meant to be used directly by any tool that needs exact, verifiable
access to DEX structures: class tables, encoded members, code items with
tries and handlers, every instruction format and payload, encoded values and
annotations, debug info, call sites, method handles and hidden-API flags. DEX
041 containers, which hold several DEX files in one buffer, are supported
through `Sections` and `ParseAt`.

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

A DEX 041 container stores several files back to back; every offset inside
them is relative to the container start, so each section is parsed over the
whole buffer:

```go
secs, err := dexfile.Sections(data) // header offsets; [0] for earlier formats
for _, off := range secs {
	f, err := dexfile.ParseAt(data, off)
	// ...
}
```

`example_test.go` is the runnable form of the first snippet. See `go doc
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
* A DEX 041 container is handled section by section: `Sections` lists the
  header offsets (the last section ends at `container_size`) and `ParseAt`
  parses one file over the whole buffer, since offsets are container-relative
  and tables such as the string pool are shared between sections.
* Instruction boundaries come from a sweep that classifies each instruction
  without decoding operands; `DecodeInsn` uses the same classifier, so the two
  never disagree. Widths are 32-bit, so a switch table with more than 32,767
  entries is not truncated. A `0x00` opcode whose high byte is not a payload
  identifier is a one-unit nop, as the runtime computes it.
* Member index differences in `class_data_item` accumulate in 64 bits, so a
  hostile difference cannot wrap into a valid-looking index; method handle
  types and targets are validated like every other table index.
* A parsed `File` is safe for concurrent use: code items, instruction sweeps
  and lookup tables are built at most once, and warnings reported during lazy
  parsing can be read with `WarningsSnapshot`.

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
* The same dexdump differential runs over arbitrary external files: point
  `DEXFILE_EXTRA_FIXTURES` at directories holding `<name>.dex` with
  `<name>.dexdump.txt` and `<name>.dexdump-f.txt` next to it (containers are
  compared section by section). Over the ten DEX files of a production
  Kotlin/Compose application (82 MB, 112,887 classes, 523,773 methods,
  7,009,567 instructions) every header field, class table, try block, debug
  position, local and instruction matched dexdump.
* `TestSweepMatchesDecode` checks the boundary sweep against full decoding on
  every method of every fixture, `TestConcurrentLazyParsing` runs under the
  race detector, and the whole suite passes `go test -race`.

```
go test ./...
go test -race ./...
go test . -fuzz FuzzParse -fuzztime 60s
DEXFILE_EXTRA_FIXTURES=/path/to/dumps go test -run AgainstDexdump -v .
```

## Performance

Single-threaded, measured on the ten DEX files above (AMD Ryzen 7 5700X, Go
1.27): parsing all headers, tables, strings and class data takes about 0.2 s;
parsing every code item and sweeping its instruction boundaries about 0.55 s;
decoding the operands of all 7 million instructions about 0.25 s; static
values, annotations and debug info under 0.1 s. Parsing retains the input
slice and allocates no per-field names unless a `*Fields` method asks for byte
ranges, so a full pass over the 82 MB performs under ten million allocations.

## Fixtures

`testdata/dex` is generated from the Java sources under `testdata/java` by
`testdata/build.sh` (needs a JDK and Android build-tools `d8`, `lib/d8.jar`
and `dexdump`); `testdata/README.md` describes every fixture, including the
DEX 041 container ones. The committed files are the ones the tests run
against, so the build script is only needed to change them.

## Non-goals

No decompilation, no APK modification, no execution of analysed code, no
ODEX/VDEX/CDEX parsing. Cross-reference indexes, control-flow graphs and
data-flow live in [dex-mcp](https://github.com/Arsylk/dex-mcp).
