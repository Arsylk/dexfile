# Upstream audit: `github.com/grassto/androguard-go`

Revision audited: `221250c` (module version `v0.0.0-20260507034903-221250c57761`),
packages `core/dex`, `core/analysis`, `internal/leb128`.

The [dex-mcp specification](https://github.com/Arsylk/dex-mcp/blob/main/SPEC.md) names `androguard-go` as the initial parser foundation and
asks for a capability audit before relying on it. The audit below records, per
area, whether the upstream implementation is **full**, **partial**, **absent**
or **untested**, with the evidence that drove each verdict. Evidence comes from
reading the code and from the differential tests in `internal/upstream`, which
run both parsers over the fixtures in `testdata/dex` and classify every
disagreement.

## Decision

The core parser in this module (`dexfile`) is a fresh implementation.
`androguard-go` remains a module dependency and is used as a differential
oracle (`internal/upstream/upstream_test.go`), alongside AOSP `dexdump`
(`dexfile_test.go`). The reasons, each tied to a requirement
in the specification:

1. **Exact byte ranges are a core requirement (§4, §6).** Upstream records no
   byte range for any structure. Encoded members, code items, debug items,
   annotations and encoded values are decoded into values with no offsets, so
   `dex.get_structure_bytes` and `dex.get_structures_at_offset` cannot be built
   on top of it without re-walking every ULEB128 sequence anyway.
2. **Malformed input must produce controlled errors (§3, §13).** Upstream slices
   `d.raw[off:]` without bounds checks in `parseClassData`, `ParseDebugInfo`,
   `parseEncodedValue`, `parseEncodedArray`, `ParseEncodedCatchHandlerList` and
   `GetProtoSignature`; `parseStringIDs` allocates `string_ids_size` entries
   straight from the header. On `testdata/dex/malformed/huge-sizes.dex`
   (`string_ids_size = 0xFFFFFFFF`) the Go runtime aborts with an
   unrecoverable out-of-memory, which is why the differential test skips that
   file for upstream. `leb128.ReadULEB128` has no length limit and silently
   returns on truncation.
3. **Instruction decoding must be exact (§7).** The differential test found the
   defects listed under *Instruction decoding* below; several change operand
   values or instruction boundaries, which would corrupt every downstream
   analysis.

Nothing was rewritten merely because it was not exposed by the CLI: the
upstream library API was inspected first, and the items marked **full** below
match our output exactly in the differential tests.

## Audit table

| Area | Verdict | Evidence |
| --- | --- | --- |
| Header (0x70) and section boundaries | **full** for v035–v039 headers | Fields agree with `dexdump -f` and with ours. DEX 041 container fields (`container_size`, `header_offset`) are not read. `header_size` is not validated. |
| string_ids / string_data | **partial** | Offsets and sizes agree. MUTF-8 surrogate pairs are combined with a wrong formula: `"😀"` decodes as `"𑨀"` (4 strings differ in `v035.dex`, classified `mutf8_surrogate_pair_decoding`). `utf16_size` is read but never checked. Lone surrogates and invalid bytes are silently dropped. |
| type_ids, proto_ids, field_ids, method_ids, class_defs | **full** | Tables identical in `TestTablesAgree`. No index validation (a `type_idx` beyond the table yields `""`). |
| class_data_item, encoded fields/methods | **partial** | Values agree on well-formed input. No bounds checks; an absurd member count panics. No byte ranges. |
| static_values / encoded_array / encoded_value | **partial** | Integer kinds correct. `VALUE_FLOAT`/`VALUE_DOUBLE` are decoded as integers cast to float (the DEX right-zero-extension is not applied). `VALUE_METHOD_TYPE` (0x15) and `VALUE_METHOD_HANDLE` (0x16) are **absent**. `VALUE_ANNOTATION` consumed size is "approximate" (`1 + len(elements)*2`), so any array containing a nested annotation desynchronises. |
| Annotations (directory, set ref list, set, item) | **partial** | Structure walked correctly for simple cases; element values suffer from the encoded_value defects above; no ranges. |
| code_item, tries, catch handlers | **partial** | Header and `insns` agree. `try_item.handler_off` is used as an *index* into the handler list instead of a byte offset into `encoded_catch_handler_list` (`code.go:118`). `TestHandlerLinking`: 0 of 20 try blocks in `v035.dex` link to the same handler as dexdump/ours. `parseCodeItem` (in `dex.go`) also pads before tries only when `off%4 != 0` using a 2-byte skip, which is correct only by coincidence of 4-byte alignment. |
| Instruction formats | **partial** | See *Instruction decoding*. |
| Switch / array payloads | **absent** in the disassembler | `ParsePackedSwitchPayload` etc. exist but `DisassembleCode` never calls them: payload bytes are decoded as a stream of instructions (`payload_decoded_as_instructions`: 21 cases per fixture). `DetermineNext` therefore cannot produce switch edges. |
| Debug info / LEB128 | **partial** | State machine implemented (`debug.go`); no bounds checks; locals of parameters are not seeded (dexdump and the runtime do), zero-length locals are kept. `uleb128p1` handled. `ReadULEB128` accepts unbounded length. |
| call_site_ids, method_handles | **absent** | Map item types are declared (`TypeCallSiteIDItem`, `TypeMethodHandleItem`) but nothing parses them; `Kind` `KindCALL_SITE` only labels operands. |
| DEX 038/039 opcodes | **wrong numbering** | Upstream places `invoke-polymorphic` at 0xee, `invoke-custom` at 0xf0, `const-method-handle` at 0xf2, `const-method-type` at 0xf3 and marks 0xfa as "invoke-polymorphic/jumbo". The specification and dexdump define 0xfa–0xff (`dex038_opcode_numbering_differs`: 17 instructions in `v038.dex`). |
| hiddenapi_class_data_item (039+) | **absent** | Type declared in `dex_enhanced.go`, never parsed. |
| ODEX | present | Out of scope here; this module rejects `dey\n` with a clear error. |
| Multidex / duplicate definitions | **absent** at the dex layer | `core/apk` loads `classes*.dex` separately; no cross-DEX merge or conflict handling. |
| Cross-references, basic blocks, CFG | **partial** | `core/analysis` builds per-method xrefs by name strings, not table indices; blocks are split on branches but switch targets and exception edges are missing because payloads/handlers are not decoded. |
| Malformed input handling | **absent** | See *Decision* (2). `TestMalformedBehaviour` logs behaviour per file. |

## Instruction decoding defects found by the differential test

Counts are from `go test ./internal/upstream -v` on `v035.dex` / `v038.dex`.

| Category | Count (v035 / v038) | Detail |
| --- | --- | --- |
| `payload_decoded_as_instructions` | 21 / 21 | No payload handling in `DisassembleCode`. |
| `const_wide_high16_shift` | 29 / 31 | Format 21h: `const-wide/high16` literal shifted by 16 instead of 48 (`disasm.go`, `Format21h` branch treats 0x19 like `const/high16`). |
| `format_35c_register_order_4_5_args` | 13 / 15 | Format 35c: registers E and F are swapped (`case 3` reads `data[5]>>4`, `case 4` reads `data[5]&0x0f`). Affects every 4- or 5-argument invoke/filled-new-array. |
| `format_45cc_4rcc_unsupported_upstream` | – / 6 | Formats 45cc/4rcc fall into the `default` branch and consume 2 bytes instead of 8, desynchronising the rest of the method. |
| `dex038_opcode_numbering_differs` | – / 17 | Wrong opcode values for 0xfa–0xff. |
| `upstream_missing_instruction` | – / 11 | Instructions following a mis-sized 45cc/4rcc or payload are lost. |
| `rsub_int_mnemonic_naming` | 1 / 1 | 0xd1 is named `rsub-int/lit16`; the specification and dexdump name it `rsub-int`. Cosmetic. |
| agreeing instructions | 49 491 / 49 243 | All other instructions agree on opcode and size. |

## What this module does differently

* Every parsed structure carries `Range{Off, Len}`; the same parse code can
  record per-field byte ranges (`FieldView`) for hex inspection.
* All reads go through a bounds-checked cursor; LEB128 is limited to 5 bytes;
  counts are validated against the remaining bytes before allocation; nesting
  depth of encoded values is bounded; a `recover` guard at the parse entry
  point turns any residual panic into an error.
* Complete opcode table (0xfa–0xff per spec), formats 45cc/4rcc, payload
  decoding with size validation, correct 35c register order, correct 21h
  shift, try/handler linking by byte offset, float/double encoded values,
  method-type/method-handle values, call sites, method handles, hidden-API
  flags, DEX 041 header fields.
* Verified against AOSP `dexdump` for every instruction of every fixture
  (offsets, bytes, mnemonic, registers, literals, targets, reference indices),
  every try/catch table, every debug position and local, every call site and
  method handle; fuzzed (`FuzzParse`, `FuzzDecodeInsn`) and exhaustively
  truncated/bit-flipped (`TestTruncationNeverPanics`, `TestBitflipsNeverPanic`).

## Items still untested here

* DEX 040/041 fixtures: `d8` in build-tools 37 emits at most version 039 for
  the fixture sources, so 040/041 parsing (notably the container header) is
  implemented from the specification but not exercised by a real file.
* Big-endian DEX is rejected rather than parsed.
* ODEX/VDEX/CDEX are out of scope.
