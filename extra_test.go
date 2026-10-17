package dexfile

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestSweepMatchesDecode checks that the boundary-only sweep agrees with full
// decoding on every method of every fixture, including container sections.
func TestSweepMatchesDecode(t *testing.T) {
	names := append(append([]string{}, fixtureDexes...), "container041")
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(fixtureDir, name+".dex"))
		if err != nil {
			t.Logf("fixture missing: %v", err)
			continue
		}
		secs, err := Sections(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, off := range secs {
			f, err := ParseAt(data, off)
			if err != nil {
				t.Fatalf("%s@%#x: %v", name, off, err)
			}
			methods, insns := 0, 0
			for i := range f.ClassDefs {
				cd := f.ClassData(uint32(i))
				if cd == nil {
					continue
				}
				for _, lst := range [][]EncodedMethod{cd.DirectMethods, cd.VirtualMethods} {
					for _, m := range lst {
						if m.CodeOff == 0 {
							continue
						}
						code, err := f.Code(m.CodeOff)
						if err != nil {
							t.Fatalf("%s: %v", f.MethodDesc(m.MethodIdx), err)
						}
						recs, err := code.InsnRecs(f)
						if err != nil {
							t.Fatalf("%s: sweep: %v", f.MethodDesc(m.MethodIdx), err)
						}
						raw := code.Insns(f)
						var pos uint32
						for _, r := range recs {
							in, err := DecodeInsn(raw, r.Off)
							if err != nil {
								t.Fatalf("%s @%#x: %v", f.MethodDesc(m.MethodIdx), r.Off, err)
							}
							if r.Off != pos || in.Off != pos || in.Units != r.Units || in.Op != r.Op || in.Payload != r.Payload {
								t.Fatalf("%s @%#x: sweep %+v vs decode off=%d units=%d op=%#x payload=%d", f.MethodDesc(m.MethodIdx), pos, r, in.Off, in.Units, in.Op, in.Payload)
							}
							pos += r.Units
							insns++
						}
						if pos*2 != uint32(len(raw)) {
							t.Fatalf("%s: sweep covers %d of %d code units", f.MethodDesc(m.MethodIdx), pos, len(raw)/2)
						}
						methods++
					}
				}
			}
			t.Logf("%s@%#x: %d methods, %d instructions", name, off, methods, insns)
		}
	}
}

// TestLargeSwitchPayload: a packed-switch table with more than 32767 targets
// is wider than 65535 code units and must not be truncated.
func TestLargeSwitchPayload(t *testing.T) {
	const n = 40000
	var buf []byte
	buf = append(buf, 0x2b, 0x00, 0x03, 0x00, 0x00, 0x00) // packed-switch v0, +3
	buf = append(buf, 0x00, 0x01)                         // payload ident 0x0100
	buf = binary.LittleEndian.AppendUint16(buf, n)
	buf = binary.LittleEndian.AppendUint32(buf, 7) // first key
	for i := 0; i < n; i++ {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(i+10))
	}
	buf = append(buf, 0x0e, 0x00) // return-void
	recs, err := SweepInsns(buf)
	if err != nil {
		t.Fatal(err)
	}
	wantUnits := uint32(4 + 2*n)
	if len(recs) != 3 || recs[1].Payload != PayloadPacked || recs[1].Units != wantUnits || recs[2].Off != 3+wantUnits || recs[2].Op != 0x0e {
		t.Fatalf("sweep: %+v", recs)
	}
	in, err := DecodeInsn(buf, 3)
	if err != nil {
		t.Fatal(err)
	}
	if in.Units != wantUnits || len(in.SwitchTargets) != n || in.PackedFirstKey != 7 || in.SwitchTargets[n-1] != n+9 {
		t.Fatalf("decode: units=%d targets=%d first=%d", in.Units, len(in.SwitchTargets), in.PackedFirstKey)
	}
	if k := FindInsn(recs, 50000); k != 1 {
		t.Fatalf("FindInsn inside the payload: got %d", k)
	}
	sw, _ := DecodeInsn(buf, 0)
	if targets := sw.BranchTargets(&in); len(targets) != n || targets[0] != 10 {
		t.Fatalf("branch targets: %d first %d", len(targets), targets[0])
	}
}

// TestNopHighByte: a 0x00 opcode with a non-payload high byte is a one-unit
// nop, as the runtime decodes it, not an error.
func TestNopHighByte(t *testing.T) {
	buf := []byte{0x00, 0x05, 0x0e, 0x00}
	in, err := DecodeInsn(buf, 0)
	if err != nil || in.Units != 1 || in.Payload != PayloadNone || in.Name() != "nop" {
		t.Fatalf("got %+v, %v", in, err)
	}
	recs, err := SweepInsns(buf)
	if err != nil || len(recs) != 2 || recs[1].Op != 0x0e {
		t.Fatalf("sweep: %+v, %v", recs, err)
	}
	for _, bad := range [][]byte{{0x00, 0x01, 0x00, 0x00}, {0x00, 0x02}, {0x00, 0x03, 0x01, 0x00}} {
		if _, err := DecodeInsn(bad, 0); err == nil {
			t.Fatalf("truncated payload %x accepted", bad)
		}
	}
}

// TestClassDataIndexWrap: member index differences accumulate without
// wrapping, and a repeated index is reported.
func TestClassDataIndexWrap(t *testing.T) {
	f := &File{Header: Header{FieldIDsSize: 10, MethodIDsSize: 10}}
	// static_fields_size=2: field 5 then a difference of 0xffffffff.
	f.data = []byte{2, 0, 0, 0, 5, 1, 0xff, 0xff, 0xff, 0xff, 0x0f, 1}
	if _, err := f.parseClassData(0, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wrapped index accepted: %v", err)
	}
	f = &File{Header: Header{FieldIDsSize: 10, MethodIDsSize: 10}}
	f.data = []byte{2, 0, 0, 0, 5, 1, 0, 1}
	cd, err := f.parseClassData(0, nil)
	if err != nil || len(cd.StaticFields) != 2 || cd.StaticFields[1].FieldIdx != 5 {
		t.Fatalf("repeated index: %+v, %v", cd, err)
	}
	if len(f.Warnings) != 1 || !strings.Contains(f.Warnings[0].What, "repeats") {
		t.Fatalf("expected one warning, got %+v", f.Warnings)
	}
}

// TestMethodHandleValidation: handle types and target indexes are checked.
func TestMethodHandleValidation(t *testing.T) {
	data, f := loadFixture(t, "v038")
	if len(f.MethodHandles) == 0 {
		t.Skip("fixture has no method handles")
	}
	r := f.MethodHandleRange(0)
	bad := append([]byte(nil), data...)
	binary.LittleEndian.PutUint16(bad[r.Off+4:], 0xffff) // field_or_method_id
	if _, err := Parse(bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("out-of-range handle target accepted: %v", err)
	}
	bad = append([]byte(nil), data...)
	binary.LittleEndian.PutUint16(bad[r.Off:], 9) // method_handle_type
	if _, err := Parse(bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown handle type accepted: %v", err)
	}
}

// TestVersionDigits: the version must be three ASCII digits.
func TestVersionDigits(t *testing.T) {
	data, _ := loadFixture(t, "tiny")
	for _, v := range []string{"0a5", "+35", " 35", "03\x00"} {
		bad := append([]byte(nil), data...)
		copy(bad[4:7], v)
		if _, err := Parse(bad); !errors.Is(err, ErrBadMagic) {
			t.Fatalf("version %q accepted: %v", v, err)
		}
	}
}

// TestConcurrentLazyParsing: lazily parsed structures are safe to use from
// many goroutines and every goroutine sees the same code item.
func TestConcurrentLazyParsing(t *testing.T) {
	_, f := loadFixture(t, "v035")
	var offs []uint32
	for i := range f.ClassDefs {
		if cd := f.ClassData(uint32(i)); cd != nil {
			for _, m := range append(cd.DirectMethods, cd.VirtualMethods...) {
				if m.CodeOff != 0 {
					offs = append(offs, m.CodeOff)
				}
			}
		}
	}
	const workers = 8
	results := make([][]*CodeItem, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for _, off := range offs {
				code, err := f.Code(off)
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := code.InsnRecs(f); err != nil {
					t.Error(err)
				}
				if code.DebugInfoOff != 0 {
					f.DebugInfo(code.DebugInfoOff, code.InsnsSize)
				}
				results[w] = append(results[w], code)
			}
			f.TypeIndex("Ljava/lang/Object;")
			f.Structures()
			f.WarningsSnapshot()
		}(w)
	}
	wg.Wait()
	for w := 1; w < workers; w++ {
		for i := range results[0] {
			if results[w][i] != results[0][i] {
				t.Fatalf("worker %d got a different *CodeItem for %#x", w, offs[i])
			}
		}
	}
}

// TestSections covers container enumeration and ParseAt.
func TestSections(t *testing.T) {
	data, _ := loadFixture(t, "tiny")
	if secs, err := Sections(data); err != nil || len(secs) != 1 || secs[0] != 0 {
		t.Fatalf("tiny: %v %v", secs, err)
	}
	if _, err := ParseAt(data, 4); err == nil {
		t.Fatal("pre-041 file parsed at a nonzero offset")
	}
	// A pre-041 header placed after the start of the buffer has file-relative
	// offsets and must be rejected even though its magic is intact.
	shifted := append(make([]byte, 16), data...)
	if _, err := ParseAt(shifted, 16); !errors.Is(err, ErrInvalid) {
		t.Fatalf("shifted pre-041 header accepted: %v", err)
	}
	if _, err := Sections(data[:0x40]); err == nil {
		t.Fatal("truncated header accepted")
	}
	cont, err := os.ReadFile(filepath.Join(fixtureDir, "container041.dex"))
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	secs, err := Sections(cont)
	if err != nil || len(secs) != 2 || secs[0] != 0 {
		t.Fatalf("container041: %v %v", secs, err)
	}
	if _, err := Sections(cont[:len(cont)-100]); err == nil {
		t.Fatal("truncated container accepted")
	}
	if _, err := ParseAt(cont, secs[1]+2); err == nil {
		t.Fatal("misplaced header accepted")
	}
}

// TestContainer041 parses both sections of the two-section container fixture
// (740 interfaces with 90 abstract methods each plus one concrete class, so
// that one section's method_ids would overflow) and checks the shared tables.
func TestContainer041(t *testing.T) {
	cont, err := os.ReadFile(filepath.Join(fixtureDir, "container041.dex"))
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	secs, err := Sections(cont)
	if err != nil || len(secs) != 2 {
		t.Fatalf("sections: %v %v", secs, err)
	}
	dumps, err := parseDexdumpSections(filepath.Join(fixtureDir, "container041.dexdump-f.txt"))
	if err != nil {
		t.Fatalf("oracle: %v", err)
	}
	if len(dumps) != 2 {
		t.Fatalf("oracle has %d sections", len(dumps))
	}
	var files []*File
	for i, off := range secs {
		f, err := ParseAt(cont, off)
		if err != nil {
			t.Fatalf("section %d: %v", i, err)
		}
		if f.HeaderOff != off || f.Header.HeaderOffset != off || f.Version != 41 || f.Header.HeaderSize != 0x78 {
			t.Fatalf("section %d header: off=%d header_offset=%d version=%d size=%#x", i, f.HeaderOff, f.Header.HeaderOffset, f.Version, f.Header.HeaderSize)
		}
		if f.Header.ContainerSize != uint32(len(cont)) {
			t.Fatalf("section %d container_size %d != %d", i, f.Header.ContainerSize, len(cont))
		}
		checkHeaderAgainstDump(t, f, dumps[i])
		files = append(files, f)
	}
	if files[0].HeaderOff+files[0].Header.FileSize != files[1].HeaderOff || files[1].HeaderOff+files[1].Header.FileSize != uint32(len(cont)) {
		t.Fatal("sections do not tile the container")
	}
	if files[0].Header.StringIDsOff != files[1].Header.StringIDsOff || files[0].Header.StringIDsSize != files[1].Header.StringIDsSize {
		t.Fatal("expected a shared string table")
	}
	interfaces, mains, methods := 0, 0, 0
	for _, f := range files {
		for i, cd := range f.ClassDefs {
			desc := f.TypeDesc(cd.ClassIdx)
			data := f.ClassData(uint32(i))
			if data == nil {
				t.Fatalf("%s: no class data", desc)
			}
			methods += len(data.DirectMethods) + len(data.VirtualMethods)
			if cd.AccessFlags&AccInterface != 0 {
				interfaces++
				if !strings.HasPrefix(desc, "Lc/I") || len(data.VirtualMethods) != 90 || len(data.DirectMethods) != 0 {
					t.Fatalf("%s: %d virtual, %d direct methods", desc, len(data.VirtualMethods), len(data.DirectMethods))
				}
				// Members are sorted by method_id, hence by name: m0, m1, m10, ...
				names := map[string]bool{}
				for _, m := range data.VirtualMethods {
					short := f.MethodShortDesc(m.MethodIdx)
					if m.CodeOff != 0 || !strings.HasPrefix(short, "m") || !strings.HasSuffix(short, "(I)I") {
						t.Fatalf("%s: unexpected method %s (code %#x)", desc, short, m.CodeOff)
					}
					names[short] = true
				}
				for k := 0; k < 90; k++ {
					if !names["m"+itoa(k)+"(I)I"] {
						t.Fatalf("%s: method m%d missing", desc, k)
					}
				}
				continue
			}
			if desc != "Lc/Main;" {
				t.Fatalf("unexpected class %s", desc)
			}
			mains++
			for _, m := range append(data.DirectMethods, data.VirtualMethods...) {
				if m.CodeOff == 0 {
					t.Fatalf("%s has no code", f.MethodDesc(m.MethodIdx))
				}
				code, err := f.Code(m.CodeOff)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := code.InsnRecs(f); err != nil {
					t.Fatalf("%s: %v", f.MethodDesc(m.MethodIdx), err)
				}
			}
		}
	}
	if interfaces != 740 || mains != 1 || methods != 740*90+93 {
		t.Fatalf("got %d interfaces, %d Main, %d methods", interfaces, mains, methods)
	}
	// Parse alone reads the first section and must not complain about the container.
	f, err := Parse(cont)
	if err != nil || len(f.Warnings) != 0 || len(f.ClassDefs) != len(files[0].ClassDefs) {
		t.Fatalf("Parse on a container: %v %+v", err, f.Warnings)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
