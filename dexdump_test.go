package dexfile

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// dumpInsn is one instruction line from `dexdump -d`.
type dumpInsn struct {
	FileOff  uint32
	CUOff    uint32
	Mnemonic string
	HexUnits []uint16 // first code units shown (dexdump truncates with "...")
	Trunc    bool
	Text     string // operand text after the mnemonic
}

type dumpTry struct {
	Start, End uint32
	Handlers   []struct {
		Type string
		Addr uint32
	}
}

type dumpLocal struct {
	Start, End uint32
	Reg        uint32
	Name, Type string
}

type dumpMethod struct {
	Class                      string
	Name                       string
	Type                       string
	Access                     uint32
	HasCode                    bool
	CodeOff                    uint32
	Regs, Ins, Outs, InsnsSize int
	Insns                      []*dumpInsn
	Tries                      []dumpTry
	Positions                  map[uint32]int32
	Locals                     []dumpLocal
}

type dumpClass struct {
	Desc    string
	Access  uint32
	Super   string
	Methods []*dumpMethod
}

type dumpHandle struct {
	Type       string
	Target     string
	TargetType string
}

type dumpCallSite struct {
	Offset uint32
	Args   []string // "value (Kind)" as printed
}

type dump struct {
	Header    map[string]string
	Classes   []*dumpClass
	Insns     map[uint32]*dumpInsn // by file offset
	Handles   []*dumpHandle
	CallSites []*dumpCallSite
}

var (
	reInsn     = regexp.MustCompile(`^([0-9a-f]{6}): ((?:[0-9a-f]{4} ?)*)(\.\.\. )?\s*\|([0-9a-f]{4}): (\S+)\s?(.*)$`)
	reCodeHdr  = regexp.MustCompile(`^([0-9a-f]{6}):\s+\|\[([0-9a-f]{6})\] `)
	reKV       = regexp.MustCompile(`^\s*([A-Za-z_#0-9 ]+?)\s*:\s?(.*)$`)
	reTryRange = regexp.MustCompile(`^\s+0x([0-9a-f]+) - 0x([0-9a-f]+)$`)
	reHandler  = regexp.MustCompile(`^\s+(\S+) -> 0x([0-9a-f]+)$`)
	rePosition = regexp.MustCompile(`^\s+0x([0-9a-f]+) line=(-?\d+)$`)
	reLocal    = regexp.MustCompile(`^\s+0x([0-9a-f]+) - 0x([0-9a-f]+) reg=(\d+) (\S+) (\S+)`)
	reMember   = regexp.MustCompile(`^  (Static fields|Instance fields|Direct methods|Virtual methods)\s+-$`)
)

func parseDexdump(path string) (*dump, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	d := &dump{Header: map[string]string{}, Insns: map[uint32]*dumpInsn{}}
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	var cls *dumpClass
	var m *dumpMethod
	section := "" // catches, positions, locals
	memberSection := ""
	inHeader := false
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "DEX file header:") {
			inHeader = true
			continue
		}
		if inHeader {
			if line == "" {
				inHeader = false
				continue
			}
			if kv := reKV.FindStringSubmatch(line); kv != nil {
				d.Header[strings.TrimSpace(kv[1])] = kv[2]
			}
			continue
		}
		if strings.HasPrefix(line, "Class #") {
			cls = &dumpClass{}
			d.Classes = append(d.Classes, cls)
			m = nil
			section = ""
			continue
		}
		if strings.HasPrefix(line, "Method handle #") {
			cls, m = nil, nil
			d.Handles = append(d.Handles, &dumpHandle{})
			section = "handle"
			continue
		}
		if strings.HasPrefix(line, "Call site #") {
			cls, m = nil, nil
			cs := &dumpCallSite{}
			if i := strings.Index(line, "// offset "); i >= 0 {
				off, _ := strconv.ParseUint(strings.TrimSpace(line[i+10:]), 10, 32)
				cs.Offset = uint32(off)
			}
			d.CallSites = append(d.CallSites, cs)
			section = "callsite"
			continue
		}
		if section == "handle" {
			if kv := reKV.FindStringSubmatch(line); kv != nil {
				h := d.Handles[len(d.Handles)-1]
				switch strings.TrimSpace(kv[1]) {
				case "type":
					h.Type = kv[2]
				case "target":
					h.Target = kv[2]
				case "target_type":
					h.TargetType = kv[2]
				}
			}
			continue
		}
		if section == "callsite" {
			if i := strings.Index(line, "] : "); i >= 0 && strings.HasPrefix(strings.TrimSpace(line), "link_argument[") {
				cs := d.CallSites[len(d.CallSites)-1]
				cs.Args = append(cs.Args, line[i+4:])
			}
			continue
		}
		if mm := reCodeHdr.FindStringSubmatch(line); mm != nil && m != nil {
			off, _ := strconv.ParseUint(mm[2], 16, 32)
			m.CodeOff = uint32(off)
			m.HasCode = true
			section = ""
			continue
		}
		if mm := reInsn.FindStringSubmatch(line); mm != nil && m != nil {
			fo, _ := strconv.ParseUint(mm[1], 16, 32)
			cu, _ := strconv.ParseUint(mm[4], 16, 32)
			in := &dumpInsn{FileOff: uint32(fo), CUOff: uint32(cu), Mnemonic: mm[5], Trunc: mm[3] != "", Text: strings.TrimSpace(mm[6])}
			for _, h := range strings.Fields(mm[2]) {
				v, _ := strconv.ParseUint(h, 16, 16)
				// dexdump prints each code unit as big-endian hex of the LE unit? It prints bytes in file order.
				in.HexUnits = append(in.HexUnits, uint16(v))
			}
			m.Insns = append(m.Insns, in)
			d.Insns[in.FileOff] = in
			continue
		}
		switch section {
		case "catches":
			if mm := reTryRange.FindStringSubmatch(line); mm != nil {
				s, _ := strconv.ParseUint(mm[1], 16, 32)
				e, _ := strconv.ParseUint(mm[2], 16, 32)
				m.Tries = append(m.Tries, dumpTry{Start: uint32(s), End: uint32(e)})
				continue
			}
			if mm := reHandler.FindStringSubmatch(line); mm != nil && len(m.Tries) > 0 {
				a, _ := strconv.ParseUint(mm[2], 16, 32)
				t := &m.Tries[len(m.Tries)-1]
				t.Handlers = append(t.Handlers, struct {
					Type string
					Addr uint32
				}{mm[1], uint32(a)})
				continue
			}
		case "positions":
			if mm := rePosition.FindStringSubmatch(line); mm != nil {
				a, _ := strconv.ParseUint(mm[1], 16, 32)
				l, _ := strconv.ParseInt(mm[2], 10, 32)
				if m.Positions == nil {
					m.Positions = map[uint32]int32{}
				}
				m.Positions[uint32(a)] = int32(l)
				continue
			}
		case "locals":
			if mm := reLocal.FindStringSubmatch(line); mm != nil {
				s, _ := strconv.ParseUint(mm[1], 16, 32)
				e, _ := strconv.ParseUint(mm[2], 16, 32)
				r, _ := strconv.ParseUint(mm[3], 10, 32)
				m.Locals = append(m.Locals, dumpLocal{Start: uint32(s), End: uint32(e), Reg: uint32(r), Name: mm[4], Type: mm[5]})
				continue
			}
		}
		if mm := reMember.FindStringSubmatch(line); mm != nil {
			memberSection = mm[1]
			m = nil
			continue
		}
		kv := reKV.FindStringSubmatch(line)
		if kv == nil {
			continue
		}
		key, val := strings.TrimSpace(kv[1]), kv[2]
		switch key {
		case "Class descriptor":
			cls.Desc = strings.Trim(val, "'")
		case "Access flags":
			if cls != nil && m == nil {
				cls.Access = parseHexFlags(val)
			}
		case "Superclass":
			cls.Super = strings.Trim(val, "'")
		case "#0", "#1", "#2", "#3", "#4", "#5", "#6", "#7", "#8", "#9":
			// handled below by prefix
		}
		if key == "Static fields" || key == "Instance fields" || key == "Direct methods" || key == "Virtual methods" {
			memberSection = key
			continue
		}
		if strings.HasPrefix(key, "#") && strings.HasPrefix(val, "(in ") && cls != nil && strings.HasSuffix(memberSection, "methods") {
			m = &dumpMethod{Class: cls.Desc}
			cls.Methods = append(cls.Methods, m)
			section = ""
			continue
		}
		if m == nil {
			continue
		}
		switch key {
		case "name":
			m.Name = strings.Trim(val, "'")
		case "type":
			m.Type = strings.Trim(val, "'")
		case "access":
			m.Access = parseHexFlags(val)
		case "registers":
			m.Regs, _ = strconv.Atoi(val)
		case "ins":
			m.Ins, _ = strconv.Atoi(val)
		case "outs":
			m.Outs, _ = strconv.Atoi(val)
		case "insns size":
			m.InsnsSize, _ = strconv.Atoi(strings.Fields(val)[0])
		case "catches":
			section = "catches"
		case "positions":
			section = "positions"
		case "locals":
			section = "locals"
		}
	}
	return d, sc.Err()
}

func parseHexFlags(s string) uint32 {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0
	}
	v, _ := strconv.ParseUint(strings.TrimPrefix(f[0], "0x"), 16, 32)
	return uint32(v)
}

// dumpHeaderInt reads an integer header field (first token).
func (d *dump) headerInt(key string) (uint64, bool) {
	v, ok := d.Header[key]
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(strings.Fields(v)[0], 10, 64)
	return n, err == nil
}

// Fields of interest in the mnemonic: dexdump names payloads differently.
func dumpMnemonic(in *Insn) string {
	switch in.Payload {
	case PayloadPacked:
		return "packed-switch-data"
	case PayloadSparse:
		return "sparse-switch-data"
	case PayloadFill:
		return "array-data"
	}
	return in.Name()
}
