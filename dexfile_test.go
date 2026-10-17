package dexfile

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const fixtureDir = "testdata/dex"

var fixtureDexes = []string{"tiny", "v035", "v037", "v038", "v039", "release"}

func loadFixture(t testing.TB, name string) ([]byte, *File) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name+".dex"))
	if err != nil {
		t.Skipf("fixture missing: %v (run testdata/build.sh)", err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatalf("%s: parse: %v", name, err)
	}
	return data, f
}

func loadDump(t testing.TB, name string) *dump {
	t.Helper()
	d, err := parseDexdump(filepath.Join(fixtureDir, name+".dexdump.txt"))
	if err != nil {
		t.Skipf("dexdump oracle missing: %v", err)
	}
	return d
}

func TestHeaderAgainstDexdump(t *testing.T) {
	for _, name := range fixtureDexes {
		t.Run(name, func(t *testing.T) {
			_, f := loadFixture(t, name)
			d, err := parseDexdump(filepath.Join(fixtureDir, name+".dexdump-f.txt"))
			if err != nil {
				t.Skip(err)
			}
			checks := map[string]uint32{
				"file_size": f.Header.FileSize, "header_size": f.Header.HeaderSize,
				"string_ids_size": f.Header.StringIDsSize, "string_ids_off": f.Header.StringIDsOff,
				"type_ids_size": f.Header.TypeIDsSize, "type_ids_off": f.Header.TypeIDsOff,
				"proto_ids_size": f.Header.ProtoIDsSize, "proto_ids_off": f.Header.ProtoIDsOff,
				"field_ids_size": f.Header.FieldIDsSize, "field_ids_off": f.Header.FieldIDsOff,
				"method_ids_size": f.Header.MethodIDsSize, "method_ids_off": f.Header.MethodIDsOff,
				"class_defs_size": f.Header.ClassDefsSize, "class_defs_off": f.Header.ClassDefsOff,
				"data_size": f.Header.DataSize, "data_off": f.Header.DataOff,
			}
			for k, got := range checks {
				want, ok := d.headerInt(k)
				if !ok {
					t.Errorf("oracle lacks %s", k)
					continue
				}
				if uint64(got) != want {
					t.Errorf("%s: got %d want %d", k, got, want)
				}
			}
			if ver := fmt.Sprintf("%03d", f.Version); !strings.Contains(d.Header["magic"], ver) {
				t.Errorf("version %s not in magic %s", ver, d.Header["magic"])
			}
			if len(f.Warnings) != 0 {
				t.Errorf("unexpected warnings: %+v", f.Warnings)
			}
		})
	}
}

// methodByDump finds our encoded method matching a dexdump method record.
func methodByDump(f *File, dm *dumpMethod) (*EncodedMethod, uint32, bool) {
	ci, ok := f.ClassDefIndex(dm.Class)
	if !ok {
		return nil, 0, false
	}
	cd := f.ClassData(ci)
	if cd == nil {
		return nil, 0, false
	}
	for _, lst := range [][]EncodedMethod{cd.DirectMethods, cd.VirtualMethods} {
		for i := range lst {
			m := &lst[i]
			if f.String(f.MethodIDs[m.MethodIdx].NameIdx) == dm.Name && f.ProtoDesc(uint32(f.MethodIDs[m.MethodIdx].ProtoIdx)) == dm.Type {
				return m, ci, true
			}
		}
	}
	return nil, 0, false
}

func TestClassesAndMethodsAgainstDexdump(t *testing.T) {
	for _, name := range fixtureDexes {
		t.Run(name, func(t *testing.T) {
			_, f := loadFixture(t, name)
			d := loadDump(t, name)
			if len(d.Classes) != len(f.ClassDefs) {
				t.Fatalf("class count: got %d want %d", len(f.ClassDefs), len(d.Classes))
			}
			for _, dc := range d.Classes {
				ci, ok := f.ClassDefIndex(dc.Desc)
				if !ok {
					t.Errorf("class %s not found", dc.Desc)
					continue
				}
				cdef := f.ClassDefs[ci]
				if cdef.AccessFlags != dc.Access {
					t.Errorf("%s access: got %#x want %#x", dc.Desc, cdef.AccessFlags, dc.Access)
				}
				super := f.TypeDesc(cdef.SuperclassIdx)
				if cdef.SuperclassIdx == NoIndex {
					super = ""
				}
				if super != dc.Super {
					t.Errorf("%s super: got %q want %q", dc.Desc, super, dc.Super)
				}
				cd := f.ClassData(ci)
				nm := 0
				if cd != nil {
					nm = len(cd.DirectMethods) + len(cd.VirtualMethods)
				}
				if nm != len(dc.Methods) {
					t.Errorf("%s method count: got %d want %d", dc.Desc, nm, len(dc.Methods))
				}
				for _, dm := range dc.Methods {
					em, _, ok := methodByDump(f, dm)
					if !ok {
						t.Errorf("%s->%s%s not found", dm.Class, dm.Name, dm.Type)
						continue
					}
					if em.AccessFlags != dm.Access {
						t.Errorf("%s->%s access: got %#x want %#x", dm.Class, dm.Name, em.AccessFlags, dm.Access)
					}
					if (em.CodeOff != 0) != dm.HasCode {
						t.Errorf("%s->%s has code: got %v want %v", dm.Class, dm.Name, em.CodeOff != 0, dm.HasCode)
						continue
					}
					if !dm.HasCode {
						continue
					}
					if em.CodeOff != dm.CodeOff {
						t.Errorf("%s->%s code off: got %#x want %#x", dm.Class, dm.Name, em.CodeOff, dm.CodeOff)
					}
					code, err := f.Code(em.CodeOff)
					if err != nil {
						t.Errorf("%s->%s code: %v", dm.Class, dm.Name, err)
						continue
					}
					if int(code.RegistersSize) != dm.Regs || int(code.InsSize) != dm.Ins || int(code.OutsSize) != dm.Outs || int(code.InsnsSize) != dm.InsnsSize {
						t.Errorf("%s->%s code header mismatch: %+v vs %+v", dm.Class, dm.Name, code, dm)
					}
					checkTries(t, f, code, dm)
					checkDebug(t, f, code, em, dm)
				}
			}
		})
	}
}

func checkTries(t *testing.T, f *File, code *CodeItem, dm *dumpMethod) {
	if len(code.Tries) != len(dm.Tries) {
		t.Errorf("%s->%s tries: got %d want %d", dm.Class, dm.Name, len(code.Tries), len(dm.Tries))
		return
	}
	for i, dt := range dm.Tries {
		tr := code.Tries[i]
		if tr.StartAddr != dt.Start || tr.EndAddr() != dt.End {
			t.Errorf("%s->%s try %d range: got [%#x,%#x) want [%#x,%#x)", dm.Class, dm.Name, i, tr.StartAddr, tr.EndAddr(), dt.Start, dt.End)
		}
		if tr.HandlerIdx < 0 {
			t.Errorf("%s->%s try %d: handler unresolved", dm.Class, dm.Name, i)
			continue
		}
		h := code.Handlers[tr.HandlerIdx]
		var got []string
		for _, c := range h.Catches {
			got = append(got, fmt.Sprintf("%s -> %#06x", f.TypeDesc(c.TypeIdx), c.Addr))
		}
		if h.CatchAllAddr >= 0 {
			got = append(got, fmt.Sprintf("<any> -> %#06x", h.CatchAllAddr))
		}
		var want []string
		for _, dh := range dt.Handlers {
			want = append(want, fmt.Sprintf("%s -> %#06x", dh.Type, dh.Addr))
		}
		if strings.Join(got, ";") != strings.Join(want, ";") {
			t.Errorf("%s->%s try %d handlers: got %v want %v", dm.Class, dm.Name, i, got, want)
		}
	}
}

func checkDebug(t *testing.T, f *File, code *CodeItem, em *EncodedMethod, dm *dumpMethod) {
	if code.DebugInfoOff == 0 {
		if len(dm.Positions) != 0 || len(dm.Locals) != 0 {
			t.Errorf("%s->%s: no debug info but oracle has %d positions", dm.Class, dm.Name, len(dm.Positions))
		}
		return
	}
	di, err := f.DebugInfoFor(code, em.MethodIdx, em.AccessFlags&AccStatic != 0)
	if err != nil {
		t.Errorf("%s->%s debug: %v", dm.Class, dm.Name, err)
		return
	}
	got := map[uint32]int32{}
	for _, p := range di.Positions {
		got[p.Addr] = p.Line // dexdump keeps the last line for duplicate addresses
	}
	if len(got) != len(dm.Positions) {
		t.Errorf("%s->%s positions: got %d want %d", dm.Class, dm.Name, len(got), len(dm.Positions))
	}
	for a, l := range dm.Positions {
		if got[a] != l {
			t.Errorf("%s->%s position %#x: got line %d want %d", dm.Class, dm.Name, a, got[a], l)
		}
	}
	// Locals: compare the multiset of (start,end,reg,name,type).
	want := map[string]int{}
	for _, l := range dm.Locals {
		want[fmt.Sprintf("%#x-%#x r%d %s %s", l.Start, l.End, l.Reg, l.Name, l.Type)]++
	}
	gotL := map[string]int{}
	for _, l := range di.Locals {
		name := "(null)"
		if l.NameIdx >= 0 {
			name = f.String(uint32(l.NameIdx))
		} else if l.SynthName != "" {
			name = l.SynthName
		}
		typ := "(null)"
		if l.TypeIdx >= 0 {
			typ = f.TypeDesc(uint32(l.TypeIdx))
		}
		gotL[fmt.Sprintf("%#x-%#x r%d %s %s", l.StartAddr, l.EndAddr, l.Register, name, typ)]++
	}
	for k, n := range want {
		if gotL[k] != n {
			t.Errorf("%s->%s local %q: got %d want %d", dm.Class, dm.Name, k, gotL[k], n)
		}
	}
	for k, n := range gotL {
		if want[k] != n {
			t.Errorf("%s->%s extra local %q (%d)", dm.Class, dm.Name, k, n)
		}
	}
}

var (
	reRegList = regexp.MustCompile(`\{([^}]*)\}`)
	reIntLit  = regexp.MustCompile(`#(?:int|long|char|short|byte) (-?\d+)`)
	reBranch  = regexp.MustCompile(`, ([0-9a-f]{4,8}) // ([+-][0-9a-f]+)$`)
	reKindRef = regexp.MustCompile(`// (string|type|field|method|proto|call_site|method_handle)@([0-9a-f]+)`)
)

func TestInstructionsAgainstDexdump(t *testing.T) {
	for _, name := range fixtureDexes {
		t.Run(name, func(t *testing.T) {
			data, f := loadFixture(t, name)
			d := loadDump(t, name)
			seen := 0
			for _, dc := range d.Classes {
				for _, dm := range dc.Methods {
					if !dm.HasCode {
						continue
					}
					em, _, ok := methodByDump(f, dm)
					if !ok {
						continue
					}
					code, err := f.Code(em.CodeOff)
					if err != nil {
						t.Errorf("%s->%s: %v", dm.Class, dm.Name, err)
						continue
					}
					recs, err := code.InsnRecs(f)
					if err != nil {
						t.Errorf("%s->%s: sweep: %v", dm.Class, dm.Name, err)
					}
					if len(recs) != len(dm.Insns) {
						t.Errorf("%s->%s: insn count got %d want %d", dm.Class, dm.Name, len(recs), len(dm.Insns))
						continue
					}
					insns := code.Insns(f)
					for i, di := range dm.Insns {
						rec := recs[i]
						seen++
						in, err := DecodeInsn(insns, rec.Off)
						if err != nil {
							t.Errorf("%s->%s @%#x: %v", dm.Class, dm.Name, rec.Off, err)
							continue
						}
						fo := code.InsnFileOffset(rec.Off)
						if fo != di.FileOff || rec.Off != di.CUOff {
							t.Errorf("%s->%s: offsets got (%#x,%#x) want (%#x,%#x)", dm.Class, dm.Name, fo, rec.Off, di.FileOff, di.CUOff)
						}
						if dumpMnemonic(&in) != di.Mnemonic {
							t.Errorf("%s->%s @%#x: mnemonic got %s want %s", dm.Class, dm.Name, rec.Off, dumpMnemonic(&in), di.Mnemonic)
						}
						// Raw bytes: compare the shown units against the file.
						raw := data[fo : fo+uint32(in.Units)*2]
						for k, u := range di.HexUnits {
							if k*2+1 >= len(raw) {
								t.Errorf("%s->%s @%#x: oracle shows more units than decoded size %d", dm.Class, dm.Name, rec.Off, in.Units)
								break
							}
							// dexdump prints the unit as two bytes in file order.
							gotU := uint16(raw[k*2])<<8 | uint16(raw[k*2+1])
							if gotU != u {
								t.Errorf("%s->%s @%#x: unit %d got %04x want %04x", dm.Class, dm.Name, rec.Off, k, gotU, u)
							}
						}
						if !di.Trunc && len(di.HexUnits) != int(in.Units) {
							t.Errorf("%s->%s @%#x: size got %d units want %d", dm.Class, dm.Name, rec.Off, in.Units, len(di.HexUnits))
						}
						checkOperands(t, f, &in, di, dm)
					}
				}
			}
			if seen == 0 {
				t.Fatal("no instructions compared")
			}
			t.Logf("%s: compared %d instructions", name, seen)
		})
	}
}

func checkOperands(t *testing.T, f *File, in *Insn, di *dumpInsn, dm *dumpMethod) {
	where := fmt.Sprintf("%s->%s @%#x %s", dm.Class, dm.Name, in.Off, in.Name())
	// Register lists for invoke / filled-new-array.
	if m := reRegList.FindStringSubmatch(di.Text); m != nil {
		var want []uint16
		s := strings.TrimSpace(m[1])
		if s != "" {
			if strings.Contains(s, "..") {
				parts := strings.Split(s, "..")
				a, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(parts[0], "v")))
				b, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(parts[1], "v")))
				for r := a; r <= b; r++ {
					want = append(want, uint16(r))
				}
			} else {
				for _, p := range strings.Split(s, ",") {
					r, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p), "v")))
					want = append(want, uint16(r))
				}
			}
		}
		got := in.ArgRegs()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: regs got %v want %v (%s)", where, got, want, di.Text)
		}
	}
	// Literals.
	if m := reIntLit.FindStringSubmatch(di.Text); m != nil && in.Info().Flags&FlagConst != 0 || in.Info().Flags&FlagBinaryLit != 0 {
		if m != nil {
			want, _ := strconv.ParseInt(m[1], 10, 64)
			got := in.Lit
			if !in.Info().Flags.has(FlagWideDest) && in.Fmt != Fmt51l {
				got = int64(int32(got))
			}
			if got != want {
				t.Errorf("%s: literal got %d want %d (%s)", where, got, want, di.Text)
			}
		}
	}
	// Branch / payload targets (absolute code unit offset).
	if m := reBranch.FindStringSubmatch(di.Text); m != nil && (in.Info().Flags&(FlagBranch|FlagSwitch) != 0 || in.Op == 0x26) {
		want, _ := strconv.ParseUint(m[1], 16, 32)
		got := uint32(int64(in.Off) + int64(in.Target))
		if got != uint32(want) {
			t.Errorf("%s: target got %#x want %#x (%s)", where, got, want, di.Text)
		}
	}
	// Reference indices.
	if m := reKindRef.FindStringSubmatch(di.Text); m != nil && in.HasRef() {
		want, _ := strconv.ParseUint(m[2], 16, 32)
		if in.Idx != uint32(want) {
			t.Errorf("%s: ref idx got %d want %d (%s)", where, in.Idx, want, di.Text)
		}
		if m[1] != in.Info().Ref.String() {
			t.Errorf("%s: ref kind got %s want %s", where, in.Info().Ref, m[1])
		}
	}
}

func (f OpFlags) has(b OpFlags) bool { return f&b != 0 }

func TestStringsAndTypes(t *testing.T) {
	_, f := loadFixture(t, "v035")
	if f.StringCount() != int(f.Header.StringIDsSize) {
		t.Fatal("string count")
	}
	// Every string round-trips through its byte range.
	for i := 0; i < f.StringCount(); i++ {
		r, _ := f.StringDataRange(uint32(i))
		b, err := f.Bytes(r)
		if err != nil {
			t.Fatal(err)
		}
		if b[len(b)-1] != 0 {
			t.Errorf("string %d range does not end with NUL", i)
		}
		c := newCursor(b, 0, "s")
		u := c.uleb("")
		s, n, _ := DecodeMUTF8(b[c.Pos() : len(b)-1])
		if s != f.String(uint32(i)) || uint32(n) != u {
			t.Errorf("string %d: re-decode mismatch", i)
		}
	}
	// Known MUTF-8 cases from the fixtures.
	found := 0
	for i := 0; i < f.StringCount(); i++ {
		s := f.String(uint32(i))
		if strings.ContainsRune(s, 0) {
			found |= 1
		}
		if strings.Contains(s, "😀") || strings.Contains(s, "🚀") || strings.Contains(s, "👍") {
			found |= 2
		}
	}
	if found != 3 {
		t.Errorf("expected NUL and supplementary strings in fixture, mask=%d", found)
	}
	if _, ok := f.TypeIndex("Lfixtures/Arith;"); !ok {
		t.Error("type lookup failed")
	}
}

func TestStaticValuesAndAnnotations(t *testing.T) {
	_, f := loadFixture(t, "v035")
	ci, ok := f.ClassDefIndex("Lfixtures/StaticValues;")
	if !ok {
		t.Skip("StaticValues class missing")
	}
	ea, err := f.StaticValues(ci)
	if err != nil || ea == nil {
		t.Fatalf("static values: %v", err)
	}
	kinds := map[uint8]bool{}
	for _, v := range ea.Values {
		kinds[v.Type] = true
		if v.Range.Len == 0 {
			t.Error("value without range")
		}
	}
	for _, k := range []uint8{ValueInt, ValueLong, ValueString, ValueBoolean, ValueDouble, ValueFloat, ValueChar, ValueShort, ValueByte} {
		if !kinds[k] {
			t.Errorf("static values lack %s", ValueTypeName(k))
		}
	}
	// Annotations directory of the annotated class.
	ci, ok = f.ClassDefIndex("Lfixtures/annot/Annotated;")
	if !ok {
		t.Skip("Annotated class missing")
	}
	off := f.ClassDefs[ci].AnnotationsOff
	dir, err := f.AnnotationsDirectory(off)
	if err != nil {
		t.Fatal(err)
	}
	if dir.ClassAnnotationsOff == 0 || len(dir.MethodAnnotations) == 0 || len(dir.FieldAnnotations) == 0 || len(dir.ParameterAnnotations) == 0 {
		t.Errorf("annotations directory incomplete: %+v", dir)
	}
	set, err := f.AnnotationSet(dir.ClassAnnotationsOff)
	if err != nil {
		t.Fatal(err)
	}
	sawMarker := false
	for _, o := range set.Offsets {
		ai, err := f.AnnotationItem(o)
		if err != nil {
			t.Fatal(err)
		}
		if f.TypeDesc(ai.Annotation.TypeIdx) == "Lfixtures/annot/Marker;" {
			sawMarker = true
			types := map[uint8]bool{}
			for _, e := range ai.Annotation.Elements {
				types[e.Value.Type] = true
			}
			for _, k := range []uint8{ValueInt, ValueString, ValueType, ValueEnum, ValueArray, ValueAnnotation} {
				if !types[k] {
					t.Errorf("Marker lacks element type %s", ValueTypeName(k))
				}
			}
		}
		if ai.Range.Len == 0 {
			t.Error("annotation without range")
		}
	}
	if !sawMarker {
		t.Error("Marker annotation not found on Annotated")
	}
}

func TestCallSitesAndMethodHandles(t *testing.T) {
	_, f := loadFixture(t, "v038")
	if len(f.CallSiteIDs) == 0 || len(f.MethodHandles) == 0 {
		t.Fatalf("v038 fixture should have call sites (%d) and method handles (%d)", len(f.CallSiteIDs), len(f.MethodHandles))
	}
	d := loadDump(t, "v038")
	if len(d.Handles) != len(f.MethodHandles) {
		t.Errorf("method handle count: got %d want %d", len(f.MethodHandles), len(d.Handles))
	}
	for i, dh := range d.Handles {
		if i >= len(f.MethodHandles) {
			break
		}
		mh := f.MethodHandles[i]
		if MethodHandleTypeName(mh.Type) != dh.Type {
			t.Errorf("handle %d type: got %s want %s", i, MethodHandleTypeName(mh.Type), dh.Type)
		}
		var target string
		if mh.IsFieldHandle() {
			fd := f.FieldIDs[mh.FieldOrMethodID]
			target = f.TypeDesc(uint32(fd.ClassIdx)) + " " + f.String(fd.NameIdx)
		} else {
			md := f.MethodIDs[mh.FieldOrMethodID]
			target = f.TypeDesc(uint32(md.ClassIdx)) + " " + f.String(md.NameIdx)
			pd := f.ProtoDesc(uint32(md.ProtoIdx))
			// dexdump renders instance handles with the receiver as first parameter.
			withRecv := "(" + f.TypeDesc(uint32(md.ClassIdx)) + pd[1:]
			if pd != dh.TargetType && withRecv != dh.TargetType {
				t.Errorf("handle %d target type: got %s want %s", i, pd, dh.TargetType)
			}
		}
		if target != dh.Target {
			t.Errorf("handle %d target: got %q want %q", i, target, dh.Target)
		}
	}
	if len(d.CallSites) != len(f.CallSiteIDs) {
		t.Errorf("call site count: got %d want %d", len(f.CallSiteIDs), len(d.CallSites))
	}
	for i, dc := range d.CallSites {
		if i >= len(f.CallSiteIDs) {
			break
		}
		if f.CallSiteIDs[i] != dc.Offset {
			t.Errorf("call site %d offset: got %d want %d", i, f.CallSiteIDs[i], dc.Offset)
		}
		cs, err := f.CallSite(uint32(i))
		if err != nil {
			t.Fatal(err)
		}
		if len(cs.Values) != len(dc.Args) {
			t.Errorf("call site %d: %d args want %d", i, len(cs.Values), len(dc.Args))
			continue
		}
		// Check the first three link arguments precisely.
		if got := fmt.Sprintf("%d (MethodHandle)", cs.Values[0].Index); got != dc.Args[0] {
			t.Errorf("call site %d arg0: got %q want %q", i, got, dc.Args[0])
		}
		if got := f.String(cs.Values[1].Index) + " (String)"; got != dc.Args[1] {
			t.Errorf("call site %d arg1: got %q want %q", i, got, dc.Args[1])
		}
		if got := f.ProtoDesc(cs.Values[2].Index) + " (MethodType)"; got != dc.Args[2] {
			t.Errorf("call site %d arg2: got %q want %q", i, got, dc.Args[2])
		}
	}
	for i := range f.CallSiteIDs {
		cs, err := f.CallSite(uint32(i))
		if err != nil {
			t.Fatalf("call site %d: %v", i, err)
		}
		if len(cs.Values) < 3 || cs.Values[0].Type != ValueMethodHandle || cs.Values[1].Type != ValueString || cs.Values[2].Type != ValueMethodType {
			t.Errorf("call site %d has unexpected shape", i)
		}
	}
	for _, mh := range f.MethodHandles {
		if mh.Type > MHInvokeInterface {
			t.Errorf("bad method handle type %d", mh.Type)
		}
	}
	// invoke-polymorphic decodes with a proto index.
	found := false
	for ci := range f.ClassDefs {
		cd := f.ClassData(uint32(ci))
		if cd == nil {
			continue
		}
		for _, m := range append(cd.DirectMethods, cd.VirtualMethods...) {
			if m.CodeOff == 0 {
				continue
			}
			code, _ := f.Code(m.CodeOff)
			recs, _ := code.InsnRecs(f)
			for _, r := range recs {
				if r.Op == 0xfa || r.Op == 0xfb {
					in, err := code.Decode(f, r.Off)
					if err != nil || in.Units != 4 || int(in.Idx2) >= len(f.ProtoIDs) {
						t.Errorf("invoke-polymorphic decode: %+v %v", in, err)
					}
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("no invoke-polymorphic found in v038")
	}
}

func TestStructuresAt(t *testing.T) {
	_, f := loadFixture(t, "tiny")
	all := f.Structures()
	if len(all) == 0 {
		t.Fatal("no structures")
	}
	// Header
	got := f.StructuresAt(0x20)
	if len(got) == 0 || got[0].Kind != TypeHeaderItem {
		t.Errorf("header not found at 0x20: %+v", got)
	}
	// An instruction inside main.
	ci, _ := f.ClassDefIndex("Lfixtures/tiny/Hello;")
	cd := f.ClassData(ci)
	var code *CodeItem
	for _, m := range cd.DirectMethods {
		if f.String(f.MethodIDs[m.MethodIdx].NameIdx) == "main" {
			code, _ = f.Code(m.CodeOff)
		}
	}
	if code == nil {
		t.Fatal("main not found")
	}
	off := code.InsnFileOffset(4) + 1 // inside invoke-virtual at cu 4
	chain := f.StructuresAt(off)
	kinds := map[uint16]bool{}
	for _, s := range chain {
		kinds[s.Kind] = true
	}
	for _, k := range []uint16{KindSection, TypeCodeItem, KindInsns, KindInstruction} {
		if !kinds[k] {
			t.Errorf("missing %s in chain %+v", StructKindName(k), chain)
		}
	}
	for _, s := range chain {
		if s.Kind == KindInstruction && (s.Index != 4 || s.Range.Len != 6) {
			t.Errorf("instruction ref wrong: %+v", s)
		}
	}
	// Every registered range is within the file and non-empty.
	for _, s := range all {
		if s.Range.Len == 0 || uint64(s.Range.End()) > uint64(f.Size()) {
			t.Errorf("bad range %+v", s)
		}
	}
}

func TestMalformedInputs(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(fixtureDir, "malformed"))
	if err != nil {
		t.Skip(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(fixtureDir, "malformed", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		f, err := Parse(data)
		switch e.Name() {
		case "bad-endian-tag.dex":
			// Either rejected (big-endian) or accepted with a warning.
			if err == nil && len(f.Warnings) == 0 {
				t.Errorf("%s: expected a warning or error", e.Name())
			}
		case "wrong-file-size.dex":
			// Non-fatal: parse succeeds with a warning.
			if err != nil {
				t.Errorf("%s: expected lenient parse, got %v", e.Name(), err)
			} else if len(f.Warnings) == 0 {
				t.Errorf("%s: expected a warning", e.Name())
			}
		default:
			if err == nil {
				t.Errorf("%s: expected error", e.Name())
			} else {
				var pe *ParseError
				if !asParseError(err, &pe) {
					t.Errorf("%s: error is not *ParseError: %T", e.Name(), err)
				}
			}
		}
	}
}

func asParseError(err error, target **ParseError) bool {
	for err != nil {
		if pe, ok := err.(*ParseError); ok {
			*target = pe
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// Truncating a valid file at every length must never panic, and must either
// fail cleanly or succeed with the data that is present.
func TestTruncationNeverPanics(t *testing.T) {
	data, _ := loadFixture(t, "tiny")
	for n := 0; n <= len(data); n++ {
		f, err := Parse(data[:n])
		if err == nil {
			exerciseAll(f)
		}
	}
}

// Flipping bytes must never panic.
func TestBitflipsNeverPanic(t *testing.T) {
	data, _ := loadFixture(t, "tiny")
	for i := 0; i < len(data); i++ {
		for _, x := range []byte{0xff, 0x80, 0x01, 0x00} {
			buf := bytes.Clone(data)
			buf[i] ^= x
			if f, err := Parse(buf); err == nil {
				exerciseAll(f)
			}
		}
	}
}

// exerciseAll touches every lazily parsed structure to surface panics.
func exerciseAll(f *File) {
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Sprintf("panic exercising parsed file: %v", r))
		}
	}()
	_ = f.Structures()
	for ci := range f.ClassDefs {
		cd := f.ClassData(uint32(ci))
		if cd == nil {
			continue
		}
		for _, m := range append(cd.DirectMethods, cd.VirtualMethods...) {
			if m.CodeOff == 0 {
				continue
			}
			code, err := f.Code(m.CodeOff)
			if err != nil {
				continue
			}
			recs, _ := code.InsnRecs(f)
			for _, r := range recs {
				code.Decode(f, r.Off)
			}
			if code.DebugInfoOff != 0 {
				f.DebugInfo(code.DebugInfoOff, code.InsnsSize)
			}
		}
		f.StaticValues(uint32(ci))
		if off := f.ClassDefs[ci].AnnotationsOff; off != 0 {
			if dir, err := f.AnnotationsDirectory(off); err == nil {
				if dir.ClassAnnotationsOff != 0 {
					if set, err := f.AnnotationSet(dir.ClassAnnotationsOff); err == nil {
						for _, o := range set.Offsets {
							f.AnnotationItem(o)
						}
					}
				}
			}
		}
	}
	for i := range f.CallSiteIDs {
		f.CallSite(uint32(i))
	}
	for i := range f.ProtoIDs {
		f.ProtoDesc(uint32(i))
	}
	for i := 0; i < 64 && i < f.StringCount(); i++ {
		f.StructuresAt(uint32(i) * 7)
	}
}

func TestHeaderFieldsHex(t *testing.T) {
	data, f := loadFixture(t, "tiny")
	views := f.HeaderFields()
	if len(views) < 23 {
		t.Fatalf("expected >=23 header fields, got %d", len(views))
	}
	if views[0].Name != "magic" || views[0].Len != 8 || views[0].Value != hex.EncodeToString(data[:8]) {
		t.Errorf("magic view wrong: %+v", views[0])
	}
	end := views[len(views)-1]
	if end.Off+end.Len != 0x70 {
		t.Errorf("header fields do not cover 0x70 bytes: %+v", end)
	}
}

func FuzzParse(f *testing.F) {
	for _, name := range []string{"tiny", "v038"} {
		data, err := os.ReadFile(filepath.Join(fixtureDir, name+".dex"))
		if err == nil {
			if len(data) > 8192 {
				f.Add(data[:8192])
			} else {
				f.Add(data)
			}
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		df, err := Parse(data)
		if err != nil {
			return
		}
		exerciseAll(df)
	})
}

func FuzzDecodeInsn(f *testing.F) {
	f.Add([]byte{0x12, 0x00, 0x0e, 0x00})
	f.Add([]byte{0x00, 0x01, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x07, 0x00, 0x00, 0x00})
	f.Fuzz(func(t *testing.T, insns []byte) {
		if len(insns)%2 == 1 {
			insns = insns[:len(insns)-1]
		}
		recs, _ := SweepInsns(insns)
		for _, r := range recs {
			in, err := DecodeInsn(insns, r.Off)
			if err != nil {
				t.Fatalf("record decoded in sweep but not individually: %v", err)
			}
			if in.Units != r.Units {
				t.Fatal("inconsistent size")
			}
			in.BranchTargets(nil)
			in.ArgRegs()
		}
	})
}
