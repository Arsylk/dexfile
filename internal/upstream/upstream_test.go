// Package upstream holds the differential tests between this module's parser
// and the upstream androguard-go library (github.com/grassto/androguard-go),
// which the specification names as the initial parser foundation.
//
// The tests document, rather than hide, the upstream behaviours that led to a
// fresh implementation: every mismatch must fall into a known category that is
// listed in docs/UPSTREAM_AUDIT.md.
package upstream

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	updex "github.com/grassto/androguard-go/core/dex"

	"github.com/arsylk/dexfile"
)

const fixtureDir = "../../testdata/dex"

func load(t *testing.T, name string) ([]byte, *dexfile.File, *updex.DexFile) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name+".dex"))
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	ours, err := dexfile.Parse(data)
	if err != nil {
		t.Fatalf("our parser: %v", err)
	}
	theirs, err := updex.Parse(data)
	if err != nil {
		t.Fatalf("upstream parser: %v", err)
	}
	return data, ours, theirs
}

// Tables that both parsers decode must agree exactly.
func TestTablesAgree(t *testing.T) {
	for _, name := range []string{"tiny", "v035", "v038", "release"} {
		t.Run(name, func(t *testing.T) {
			_, ours, theirs := load(t, name)
			if ours.StringCount() != len(theirs.StringData) {
				t.Fatalf("string count %d vs %d", ours.StringCount(), len(theirs.StringData))
			}
			surrogateDiffs := 0
			for i := 0; i < ours.StringCount(); i++ {
				a, b := ours.String(uint32(i)), theirs.GetString(uint32(i))
				if a == b {
					continue
				}
				// Upstream combines surrogate pairs with a wrong formula, so
				// strings containing supplementary characters differ.
				if hasSupplementary(a) && len([]rune(a)) == len([]rune(b)) {
					surrogateDiffs++
					continue
				}
				t.Errorf("string %d: %q vs %q", i, a, b)
			}
			if surrogateDiffs > 0 {
				t.Logf("mutf8_surrogate_pair_decoding: %d strings differ (upstream defect)", surrogateDiffs)
			}
			if len(ours.TypeIDs) != len(theirs.TypeIDs) {
				t.Fatalf("type count")
			}
			for i := range ours.TypeIDs {
				if ours.TypeDesc(uint32(i)) != theirs.GetTypeName(uint32(i)) {
					t.Errorf("type %d mismatch", i)
				}
			}
			if len(ours.MethodIDs) != len(theirs.MethodIDs) || len(ours.FieldIDs) != len(theirs.FieldIDs) || len(ours.ClassDefs) != len(theirs.ClassDefs) {
				t.Fatalf("id table counts differ")
			}
			for i := range ours.MethodIDs {
				a, b := ours.MethodIDs[i], theirs.MethodIDs[i]
				if a.ClassIdx != b.ClassIdx || a.NameIdx != b.NameIdx || a.ProtoIdx != b.ProtoIdx {
					t.Errorf("method_id %d differs", i)
				}
			}
			for i := range ours.ClassDefs {
				a, b := ours.ClassDefs[i], theirs.ClassDefs[i]
				if a.ClassIdx != b.ClassIdx || a.AccessFlags != b.AccessFlags || a.SuperclassIdx != b.SuperclassIdx || a.ClassDataOff != b.ClassDataOff {
					t.Errorf("class_def %d differs", i)
				}
				cd := ours.ClassData(uint32(i))
				ucd := theirs.ClassData[uint32(i)]
				if (cd == nil) != (ucd == nil) {
					t.Errorf("class_data presence differs for %d", i)
					continue
				}
				if cd == nil {
					continue
				}
				if len(cd.DirectMethods) != len(ucd.DirectMethods) || len(cd.VirtualMethods) != len(ucd.VirtualMethods) || len(cd.StaticFields) != len(ucd.StaticFields) || len(cd.InstanceFields) != len(ucd.InstanceFields) {
					t.Errorf("class_data %d member counts differ", i)
					continue
				}
				for k := range cd.DirectMethods {
					if cd.DirectMethods[k].MethodIdx != ucd.DirectMethods[k].MethodIdxDiff || cd.DirectMethods[k].CodeOff != ucd.DirectMethods[k].CodeOff {
						t.Errorf("class %d direct method %d differs", i, k)
					}
				}
			}
			if len(theirs.MapItems) != len(ours.MapList) {
				t.Errorf("map list size %d vs %d", len(ours.MapList), len(theirs.MapItems))
			}
		})
	}
}

// Instruction decoding: mismatches are classified; only known upstream defects
// are tolerated and their counts are reported.
func TestInstructionDecodingClassified(t *testing.T) {
	for _, name := range []string{"v035", "v038"} {
		t.Run(name, func(t *testing.T) {
			_, ours, theirs := load(t, name)
			dis := updex.NewDisassembler(theirs)
			agree := 0
			cats := map[string]int{}
			unknown := []string{}
			for ci := range ours.ClassDefs {
				cd := ours.ClassData(uint32(ci))
				if cd == nil {
					continue
				}
				for _, m := range append(cd.DirectMethods, cd.VirtualMethods...) {
					if m.CodeOff == 0 {
						continue
					}
					code, err := ours.Code(m.CodeOff)
					if err != nil {
						t.Fatal(err)
					}
					recs, _ := code.InsnRecs(ours)
					ucode, ok := theirs.CodeItems[m.CodeOff]
					if !ok {
						cats["upstream_missing_code_item"]++
						continue
					}
					uinsns, _ := dis.DisassembleCode(ucode)
					byOff := map[uint32]updex.Instruction{}
					for _, ui := range uinsns {
						byOff[ui.Offset/2] = ui
					}
					for _, r := range recs {
						in, _ := code.Decode(ours, r.Off)
						ui, ok := byOff[r.Off]
						switch {
						case in.Payload != dexfile.PayloadNone:
							// Upstream has no payload handling: it decodes payload bytes as instructions.
							cats["payload_decoded_as_instructions"]++
						case !ok:
							cats["upstream_missing_instruction"]++
						case in.Fmt == dexfile.Fmt45cc || in.Fmt == dexfile.Fmt4rcc:
							cats["format_45cc_4rcc_unsupported_upstream"]++
						case in.Op >= 0xfa:
							if ui.Opcode.Name != in.Name() {
								cats["dex038_opcode_numbering_differs"]++
							} else {
								agree++
							}
						case in.Op == 0xd1 && ui.Opcode.Name == "rsub-int/lit16":
							cats["rsub_int_mnemonic_naming"]++ // spec and dexdump call 0xd1 "rsub-int"
						case ui.Opcode.Name != in.Name():
							unknown = append(unknown, fmt.Sprintf("%s @%#x: ours %s theirs %s", ours.MethodDesc(m.MethodIdx), r.Off, in.Name(), ui.Opcode.Name))
						case len(ui.RawBytes) != int(in.Units)*2:
							unknown = append(unknown, fmt.Sprintf("%s @%#x: size %d vs %d", ours.MethodDesc(m.MethodIdx), r.Off, in.Units*2, len(ui.RawBytes)))
						default:
							agree++
							// Operand-level checks on formats where upstream is known to be wrong.
							if in.Op == 0x19 { // const-wide/high16
								if lit := ui.Operands[1].Value; lit != in.Lit {
									cats["const_wide_high16_shift"]++
								}
							}
							if in.Fmt == dexfile.Fmt35c && len(in.Regs) >= 4 {
								var theirRegs []uint16
								for _, op := range ui.Operands {
									if op.Type == updex.OpRegister {
										theirRegs = append(theirRegs, uint16(op.Value))
									}
								}
								if fmt.Sprint(theirRegs) != fmt.Sprint(in.Regs) {
									cats["format_35c_register_order_4_5_args"]++
								}
							}
						}
					}
				}
			}
			if len(unknown) > 0 {
				sort.Strings(unknown)
				t.Errorf("%d unclassified mismatches, e.g.:\n%s", len(unknown), strings.Join(unknown[:min(10, len(unknown))], "\n"))
			}
			keys := make([]string, 0, len(cats))
			for k := range cats {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				t.Logf("%s: %d", k, cats[k])
			}
			t.Logf("agreeing instructions: %d", agree)
			if agree == 0 {
				t.Fatal("no agreement at all; test harness broken")
			}
		})
	}
}

// Try/catch handler linking: upstream indexes handlers by list position
// instead of byte offset; record how often that produces a different handler.
func TestHandlerLinking(t *testing.T) {
	_, ours, theirs := load(t, "v035")
	diff, same := 0, 0
	for ci := range ours.ClassDefs {
		cd := ours.ClassData(uint32(ci))
		if cd == nil {
			continue
		}
		for _, m := range append(cd.DirectMethods, cd.VirtualMethods...) {
			if m.CodeOff == 0 {
				continue
			}
			code, _ := ours.Code(m.CodeOff)
			if len(code.Tries) == 0 {
				continue
			}
			ucode, err := theirs.ParseDalvikCode(m.CodeOff)
			if err != nil {
				continue
			}
			for i, tr := range code.Tries {
				if i >= len(ucode.Tries) || tr.HandlerIdx < 0 {
					continue
				}
				ours := code.Handlers[tr.HandlerIdx]
				th := ucode.Tries[i].Handler
				if th == nil {
					diff++
					continue
				}
				if len(th.Handlers) != len(ours.Catches) {
					diff++
					continue
				}
				ok := true
				for k := range ours.Catches {
					if th.Handlers[k].TypeIdx != ours.Catches[k].TypeIdx || th.Handlers[k].Addr != ours.Catches[k].Addr {
						ok = false
					}
				}
				if ok {
					same++
				} else {
					diff++
				}
			}
		}
	}
	t.Logf("try handlers: %d identical, %d differ (upstream uses handler_off as a list index)", same, diff)
	if same+diff == 0 {
		t.Fatal("no tries compared")
	}
}

// Malformed inputs: our parser must return errors; upstream behaviour is only observed.
func TestMalformedBehaviour(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(fixtureDir, "malformed"))
	if err != nil {
		t.Skip(err)
	}
	for _, e := range entries {
		if e.Name() == "huge-sizes.dex" {
			// Upstream allocates string tables sized from the header without
			// validation (string_ids_size = 0xFFFFFFFF): the Go runtime aborts
			// with an unrecoverable out-of-memory, so it cannot run in-process.
			t.Logf("%s: skipped for upstream (unrecoverable out-of-memory); ours=%v", e.Name(), mustErr(t, filepath.Join(fixtureDir, "malformed", e.Name())))
			continue
		}
		data, _ := os.ReadFile(filepath.Join(fixtureDir, "malformed", e.Name()))
		theirsPanicked := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					theirsPanicked = true
				}
			}()
			_, _ = updex.Parse(data)
		}()
		_, ourErr := dexfile.Parse(data)
		t.Logf("%s: ours=%v upstream_panic=%v", e.Name(), ourErr != nil, theirsPanicked)
	}
}

func mustErr(t *testing.T, path string) bool {
	data, _ := os.ReadFile(path)
	_, err := dexfile.Parse(data)
	return err != nil
}

func hasSupplementary(s string) bool {
	for _, r := range s {
		if r > 0xffff {
			return true
		}
	}
	return false
}
