package dexfile

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"
)

// NoIndex is the sentinel used by several tables for "absent".
const NoIndex = 0xffffffff

// Header is the parsed header_item. Field ranges are derivable from the fixed
// layout; HeaderFields returns them explicitly.
type Header struct {
	Magic         [8]byte
	Checksum      uint32
	Signature     [20]byte
	FileSize      uint32
	HeaderSize    uint32
	EndianTag     uint32
	LinkSize      uint32
	LinkOff       uint32
	MapOff        uint32
	StringIDsSize uint32
	StringIDsOff  uint32
	TypeIDsSize   uint32
	TypeIDsOff    uint32
	ProtoIDsSize  uint32
	ProtoIDsOff   uint32
	FieldIDsSize  uint32
	FieldIDsOff   uint32
	MethodIDsSize uint32
	MethodIDsOff  uint32
	ClassDefsSize uint32
	ClassDefsOff  uint32
	DataSize      uint32
	DataOff       uint32
	// DEX 041 container fields (present only when HeaderSize >= 0x78).
	ContainerSize uint32
	HeaderOffset  uint32
}

// ProtoID is a proto_id_item.
type ProtoID struct {
	ShortyIdx     uint32
	ReturnTypeIdx uint32
	ParametersOff uint32
}

// FieldID is a field_id_item.
type FieldID struct {
	ClassIdx uint16
	TypeIdx  uint16
	NameIdx  uint32
}

// MethodID is a method_id_item.
type MethodID struct {
	ClassIdx uint16
	ProtoIdx uint16
	NameIdx  uint32
}

// ClassDef is a class_def_item.
type ClassDef struct {
	ClassIdx        uint32
	AccessFlags     uint32
	SuperclassIdx   uint32
	InterfacesOff   uint32
	SourceFileIdx   uint32
	AnnotationsOff  uint32
	ClassDataOff    uint32
	StaticValuesOff uint32
}

// EncodedField is one entry of a class_data_item field list, with the
// index difference already accumulated into FieldIdx.
type EncodedField struct {
	FieldIdx    uint32
	AccessFlags uint32
	Range       Range // bytes of this encoded_field within class_data_item
}

// EncodedMethod is one entry of a class_data_item method list.
type EncodedMethod struct {
	MethodIdx   uint32
	AccessFlags uint32
	CodeOff     uint32
	Range       Range
}

// ClassData is a parsed class_data_item.
type ClassData struct {
	Range          Range
	StaticFields   []EncodedField
	InstanceFields []EncodedField
	DirectMethods  []EncodedMethod
	VirtualMethods []EncodedMethod
}

// TypeList is a parsed type_list.
type TypeList struct {
	Range Range
	Types []uint16
}

// MapItem is one map_list entry.
type MapItem struct {
	Type   uint16
	Size   uint32
	Offset uint32
}

// MethodHandle is a method_handle_item.
type MethodHandle struct {
	Type            uint16
	FieldOrMethodID uint16
}

// Method handle types.
const (
	MHStaticPut         = 0
	MHStaticGet         = 1
	MHInstancePut       = 2
	MHInstanceGet       = 3
	MHInvokeStatic      = 4
	MHInvokeInstance    = 5
	MHInvokeConstructor = 6
	MHInvokeDirect      = 7
	MHInvokeInterface   = 8
)

var mhNames = [...]string{"static-put", "static-get", "instance-put", "instance-get", "invoke-static", "invoke-instance", "invoke-constructor", "invoke-direct", "invoke-interface"}

// MethodHandleTypeName returns a mnemonic for a method handle type.
func MethodHandleTypeName(t uint16) string {
	if int(t) < len(mhNames) {
		return mhNames[t]
	}
	return "unknown-" + strconv.Itoa(int(t))
}

// IsFieldHandle reports whether the handle refers to a field (vs. a method).
func (m MethodHandle) IsFieldHandle() bool { return m.Type <= MHInstanceGet }

type stringItem struct {
	dataOff   uint32 // string_data_item offset
	dataLen   uint32 // total item length incl. uleb size and NUL
	utf16Size uint32 // declared utf16_size
	str       string
	flags     uint8 // bit0 malformed MUTF-8, bit1 utf16 size mismatch
}

// Warning records a non-fatal inconsistency detected while parsing.
type Warning struct {
	What   string `json:"what"`
	Offset uint32 `json:"offset"`
}

// File is a parsed, immutable DEX file.
//
// For a DEX 041 container, data is the whole container and HeaderOff is the
// offset of this file's header inside it; every other offset in the file is
// relative to the container start, exactly as stored.
type File struct {
	data      []byte
	HeaderOff uint32 // offset of header_item within data (0 except in containers)
	Version   int    // 35, 37, 38, 39, 40, 41, ...
	Header    Header

	StringIDs     []uint32 // string_data_off per string
	TypeIDs       []uint32 // descriptor string idx per type
	ProtoIDs      []ProtoID
	FieldIDs      []FieldID
	MethodIDs     []MethodID
	ClassDefs     []ClassDef
	CallSiteIDs   []uint32 // call_site_off per call site
	MethodHandles []MethodHandle
	MapList       []MapItem
	MapListRange  Range

	// Warnings collects non-fatal inconsistencies. Parse fills it; lazily
	// parsed structures (code items, debug info, lookups) may append to it
	// later, so concurrent users should read it through WarningsSnapshot.
	Warnings []Warning
	warnMu   sync.Mutex

	strings      []stringItem
	classData    []*ClassData // indexed by class def; nil when class has no data or it failed
	classDataErr []error      // per class def parse error, if any

	// lazily built lookup tables
	once        sync.Once
	typeByDesc  map[string]uint32 // descriptor -> type idx
	classByType map[uint32]uint32 // type idx -> class def idx
	stringByVal map[string]uint32

	codeMu    sync.RWMutex
	codeCache map[uint32]*CodeItem

	hiddenAPI *hiddenAPIData

	structsOnce sync.Once
	structs     []StructRef
	depthStart  [4]int // index of the first entry at each depth (3 = len)
}

// Parse parses a DEX file image. The slice is retained (not copied) and must
// not be modified afterwards. For a DEX 041 container it parses the first
// section; use Sections and ParseAt to reach the others.
func Parse(data []byte) (*File, error) {
	return ParseAt(data, 0)
}

// ParseAt parses the DEX file whose header_item starts at headerOff within
// data. It exists for DEX 041 containers, where data is the whole container
// and several files follow each other, each with its own header; all offsets
// stored in such a file are relative to the container start, so the whole
// container must be passed. For earlier formats headerOff must be 0.
func ParseAt(data []byte, headerOff uint32) (f *File, err error) {
	defer func() {
		// Belt and braces: the parser is written to be bounds-safe, but a
		// panic must never escape to the caller on hostile input.
		if r := recover(); r != nil {
			f = nil
			err = perr("parser", headerOff, fmt.Errorf("internal error: %v", r))
		}
	}()
	return parseAt(data, headerOff)
}

// magicVersion validates the magic at the start of hd and returns the version.
func magicVersion(hd []byte, at uint32) (int, error) {
	if len(hd) < 8 {
		return 0, perr("header", at, ErrTruncated)
	}
	if !bytes.Equal(hd[0:4], []byte("dex\n")) || hd[7] != 0 {
		if bytes.Equal(hd[0:4], []byte("dey\n")) {
			return 0, perr("header", at, fmt.Errorf("%w: optimized DEX (ODEX) is not supported", ErrBadMagic))
		}
		return 0, perr("header", at, ErrBadMagic)
	}
	ver := 0
	for _, ch := range hd[4:7] {
		if ch < '0' || ch > '9' {
			return 0, perr("header", at+4, fmt.Errorf("%w: unknown version %q", ErrBadMagic, string(hd[4:7])))
		}
		ver = ver*10 + int(ch-'0')
	}
	if ver < 35 {
		return 0, perr("header", at+4, fmt.Errorf("%w: unknown version %q", ErrBadMagic, string(hd[4:7])))
	}
	return ver, nil
}

// isContainerHeader reports whether a header at hd has the DEX 041 container
// fields (version 041 or later with header_size >= 0x78).
func isContainerHeader(hd []byte, ver int) bool {
	return ver >= 41 && len(hd) >= 0x78 && le32(hd[36:]) >= 0x78
}

// Sections returns the header offsets of the DEX files stored in data. A DEX
// 041 container holds several files back to back, each starting with its own
// header_item and spanning its header's file_size; the last one ends at
// container_size. Every offset inside such a container is relative to the
// container start, so each section must be parsed with ParseAt over the
// whole buffer. Earlier formats and single-file containers yield one offset, 0.
func Sections(data []byte) ([]uint32, error) {
	ver, err := magicVersion(data, 0)
	if err != nil {
		return nil, err
	}
	if len(data) < 0x70 {
		return nil, perr("header", 0, ErrTruncated)
	}
	if !isContainerHeader(data, ver) {
		return []uint32{0}, nil
	}
	total := uint64(len(data))
	if cs := uint64(le32(data[0x70:])); cs >= 0x70 && cs < total {
		total = cs
	}
	var out []uint32
	for off := uint64(0); off < total; {
		if off+0x70 > total {
			return nil, perr("container", uint32(off), fmt.Errorf("%w: section %d header", ErrTruncated, len(out)))
		}
		if _, err := magicVersion(data[off:], uint32(off)); err != nil {
			return nil, perr("container", uint32(off), fmt.Errorf("section %d: %w", len(out), err))
		}
		fs := uint64(le32(data[off+32:]))
		if fs < 0x70 || off+fs > total {
			return nil, perr("container", uint32(off), fmt.Errorf("%w: section %d file_size %d", ErrInvalid, len(out), fs))
		}
		out = append(out, uint32(off))
		off += fs
	}
	return out, nil
}

func parseAt(data []byte, headerOff uint32) (*File, error) {
	if uint64(headerOff)+0x70 > uint64(len(data)) {
		return nil, perr("header", headerOff, ErrTruncated)
	}
	hd := data[headerOff:]
	ver, err := magicVersion(hd, headerOff)
	if err != nil {
		return nil, err
	}
	f := &File{data: data, HeaderOff: headerOff, Version: ver, codeCache: make(map[uint32]*CodeItem)}
	if ver > 41 {
		f.warn("untested DEX version "+strconv.Itoa(ver), headerOff+4)
	}
	if err := f.parseHeader(nil); err != nil {
		return nil, err
	}
	h := &f.Header
	if h.EndianTag != 0x12345678 {
		if h.EndianTag == 0x78563412 {
			return nil, perr("header.endian_tag", headerOff+40, fmt.Errorf("%w: big-endian DEX is not supported", ErrInvalid))
		}
		f.warn(fmt.Sprintf("unexpected endian_tag 0x%08x", h.EndianTag), headerOff+40)
	}
	if isContainerHeader(hd, ver) {
		if h.HeaderOffset != headerOff {
			f.warn(fmt.Sprintf("header_offset %d != actual %d", h.HeaderOffset, headerOff), headerOff+0x74)
		}
		if uint64(h.ContainerSize) != uint64(len(data)) {
			f.warn(fmt.Sprintf("container_size %d != actual %d", h.ContainerSize, len(data)), headerOff+0x70)
		}
		if uint64(headerOff)+uint64(h.FileSize) > uint64(len(data)) {
			f.warn(fmt.Sprintf("file_size %d exceeds the container", h.FileSize), headerOff+32)
		}
	} else {
		if headerOff != 0 {
			return nil, perr("header", headerOff, fmt.Errorf("%w: DEX %03d has no container header and must start at offset 0", ErrInvalid, ver))
		}
		if uint64(h.FileSize) != uint64(len(data)) {
			f.warn(fmt.Sprintf("header file_size %d != actual %d", h.FileSize, len(data)), 32)
		}
	}

	if err = f.parseStringIDs(); err != nil {
		return nil, err
	}
	if err = f.parseTypeIDs(); err != nil {
		return nil, err
	}
	if err = f.parseProtoIDs(); err != nil {
		return nil, err
	}
	if err = f.parseFieldIDs(); err != nil {
		return nil, err
	}
	if err = f.parseMethodIDs(); err != nil {
		return nil, err
	}
	if err = f.parseClassDefs(); err != nil {
		return nil, err
	}
	f.parseMapList() // non-fatal
	if err = f.parseCallSitesAndHandles(); err != nil {
		return nil, err
	}
	f.parseAllClassData()
	f.parseHiddenAPI()
	return f, nil
}

func (f *File) warn(what string, off uint32) {
	f.warnMu.Lock()
	if len(f.Warnings) < 256 {
		f.Warnings = append(f.Warnings, Warning{What: what, Offset: off})
	}
	f.warnMu.Unlock()
}

// WarningsSnapshot returns a copy of the warnings recorded so far. Unlike
// reading the Warnings field directly, it is safe while other goroutines are
// still parsing lazily evaluated structures of the same file.
func (f *File) WarningsSnapshot() []Warning {
	f.warnMu.Lock()
	defer f.warnMu.Unlock()
	return append([]Warning(nil), f.Warnings...)
}

// Data returns the underlying file bytes. Callers must not modify them.
func (f *File) Data() []byte { return f.data }

// Size returns the file length in bytes.
func (f *File) Size() uint32 { return uint32(len(f.data)) }

// Bytes returns the bytes of a range, or an error if it is out of bounds.
func (f *File) Bytes(r Range) ([]byte, error) {
	if err := checkRange(f.data, r.Off, uint64(r.Len), "range"); err != nil {
		return nil, err
	}
	return f.data[r.Off : r.Off+r.Len], nil
}

func (f *File) parseHeader(rec *[]FieldView) error {
	c := newCursor(f.data, f.HeaderOff, "header")
	c.rec = rec
	h := &f.Header
	copy(h.Magic[:], c.bytes("magic", 8))
	h.Checksum = c.u32("checksum")
	copy(h.Signature[:], c.bytes("signature", 20))
	h.FileSize = c.u32("file_size")
	h.HeaderSize = c.u32("header_size")
	h.EndianTag = c.u32("endian_tag")
	h.LinkSize = c.u32("link_size")
	h.LinkOff = c.u32("link_off")
	h.MapOff = c.u32("map_off")
	h.StringIDsSize = c.u32("string_ids_size")
	h.StringIDsOff = c.u32("string_ids_off")
	h.TypeIDsSize = c.u32("type_ids_size")
	h.TypeIDsOff = c.u32("type_ids_off")
	h.ProtoIDsSize = c.u32("proto_ids_size")
	h.ProtoIDsOff = c.u32("proto_ids_off")
	h.FieldIDsSize = c.u32("field_ids_size")
	h.FieldIDsOff = c.u32("field_ids_off")
	h.MethodIDsSize = c.u32("method_ids_size")
	h.MethodIDsOff = c.u32("method_ids_off")
	h.ClassDefsSize = c.u32("class_defs_size")
	h.ClassDefsOff = c.u32("class_defs_off")
	h.DataSize = c.u32("data_size")
	h.DataOff = c.u32("data_off")
	if c.Err() != nil {
		return c.Err()
	}
	if h.HeaderSize >= 0x78 && uint64(f.HeaderOff)+0x78 <= uint64(len(f.data)) {
		h.ContainerSize = c.u32("container_size")
		h.HeaderOffset = c.u32("header_offset")
	}
	if h.HeaderSize != 0x70 && h.HeaderSize != 0x78 {
		f.warn(fmt.Sprintf("unexpected header_size 0x%x", h.HeaderSize), f.HeaderOff+36)
	}
	return c.Err()
}

// HeaderRange returns the byte range of the header_item.
func (f *File) HeaderRange() Range {
	n := f.Header.HeaderSize
	if n < 0x70 || uint64(f.HeaderOff)+uint64(n) > uint64(len(f.data)) {
		n = 0x70
	}
	return Range{f.HeaderOff, n}
}

// HeaderFields returns the header fields with their byte ranges.
func (f *File) HeaderFields() []FieldView {
	var views []FieldView
	g := &File{data: f.data, HeaderOff: f.HeaderOff}
	_ = g.parseHeader(&views)
	return views
}

// checkTable validates a (size, offset, item size) triple from the header.
func (f *File) checkTable(what string, off, size uint32, itemSize uint32) error {
	if size == 0 {
		return nil
	}
	if off == 0 {
		return perr(what, 0, fmt.Errorf("%w: size %d with zero offset", ErrInvalid, size))
	}
	if uint64(off)%4 != 0 {
		return perr(what, off, ErrBadAlign)
	}
	if uint64(off)+uint64(size)*uint64(itemSize) > uint64(len(f.data)) {
		return perr(what, off, fmt.Errorf("%w: %d items of %d bytes", ErrTooLarge, size, itemSize))
	}
	return nil
}

func (f *File) parseStringIDs() error {
	h := &f.Header
	if err := f.checkTable("string_ids", h.StringIDsOff, h.StringIDsSize, 4); err != nil {
		return err
	}
	n := int(h.StringIDsSize)
	f.StringIDs = make([]uint32, n)
	f.strings = make([]stringItem, n)
	c := newCursor(f.data, h.StringIDsOff, "string_ids")
	for i := 0; i < n; i++ {
		f.StringIDs[i] = c.u32("")
	}
	if c.Err() != nil {
		return c.Err()
	}
	for i := 0; i < n; i++ {
		if err := f.parseStringData(uint32(i)); err != nil {
			return err
		}
	}
	return nil
}

func (f *File) parseStringData(idx uint32) error {
	off := f.StringIDs[idx]
	c := newCursor(f.data, off, "string_data_item")
	utf16Size := c.uleb("utf16_size")
	if c.Err() != nil {
		return perr(fmt.Sprintf("string_data_item[%d]", idx), off, c.Err())
	}
	start := int(c.Pos())
	end := bytes.IndexByte(f.data[start:], 0)
	if end < 0 {
		return perr(fmt.Sprintf("string_data_item[%d]", idx), off, fmt.Errorf("%w: missing NUL terminator", ErrTruncated))
	}
	raw := f.data[start : start+end]
	s, u16, malformed := DecodeMUTF8(raw)
	it := &f.strings[idx]
	it.dataOff = off
	it.dataLen = uint32(start+end+1) - off
	it.utf16Size = utf16Size
	it.str = s
	if malformed {
		it.flags |= 1
	}
	if uint32(u16) != utf16Size {
		it.flags |= 2
	}
	return nil
}

func (f *File) parseTypeIDs() error {
	h := &f.Header
	if err := f.checkTable("type_ids", h.TypeIDsOff, h.TypeIDsSize, 4); err != nil {
		return err
	}
	f.TypeIDs = make([]uint32, h.TypeIDsSize)
	c := newCursor(f.data, h.TypeIDsOff, "type_ids")
	for i := range f.TypeIDs {
		v := c.u32("")
		if v >= h.StringIDsSize {
			return perr("type_ids", c.Pos()-4, fmt.Errorf("%w: descriptor_idx %d >= %d strings", ErrInvalid, v, h.StringIDsSize))
		}
		f.TypeIDs[i] = v
	}
	return c.Err()
}

func (f *File) parseProtoIDs() error {
	h := &f.Header
	if err := f.checkTable("proto_ids", h.ProtoIDsOff, h.ProtoIDsSize, 12); err != nil {
		return err
	}
	f.ProtoIDs = make([]ProtoID, h.ProtoIDsSize)
	c := newCursor(f.data, h.ProtoIDsOff, "proto_ids")
	for i := range f.ProtoIDs {
		p := &f.ProtoIDs[i]
		p.ShortyIdx = c.u32("")
		p.ReturnTypeIdx = c.u32("")
		p.ParametersOff = c.u32("")
		if c.Err() == nil {
			if p.ShortyIdx >= h.StringIDsSize || p.ReturnTypeIdx >= h.TypeIDsSize {
				return perr("proto_ids", c.Pos()-12, fmt.Errorf("%w: index out of range in proto %d", ErrInvalid, i))
			}
		}
	}
	return c.Err()
}

func (f *File) parseFieldIDs() error {
	h := &f.Header
	if err := f.checkTable("field_ids", h.FieldIDsOff, h.FieldIDsSize, 8); err != nil {
		return err
	}
	f.FieldIDs = make([]FieldID, h.FieldIDsSize)
	c := newCursor(f.data, h.FieldIDsOff, "field_ids")
	for i := range f.FieldIDs {
		p := &f.FieldIDs[i]
		p.ClassIdx = c.u16("")
		p.TypeIdx = c.u16("")
		p.NameIdx = c.u32("")
		if c.Err() == nil {
			if uint32(p.ClassIdx) >= h.TypeIDsSize || uint32(p.TypeIdx) >= h.TypeIDsSize || p.NameIdx >= h.StringIDsSize {
				return perr("field_ids", c.Pos()-8, fmt.Errorf("%w: index out of range in field %d", ErrInvalid, i))
			}
		}
	}
	return c.Err()
}

func (f *File) parseMethodIDs() error {
	h := &f.Header
	if err := f.checkTable("method_ids", h.MethodIDsOff, h.MethodIDsSize, 8); err != nil {
		return err
	}
	f.MethodIDs = make([]MethodID, h.MethodIDsSize)
	c := newCursor(f.data, h.MethodIDsOff, "method_ids")
	for i := range f.MethodIDs {
		p := &f.MethodIDs[i]
		p.ClassIdx = c.u16("")
		p.ProtoIdx = c.u16("")
		p.NameIdx = c.u32("")
		if c.Err() == nil {
			if uint32(p.ClassIdx) >= h.TypeIDsSize || uint32(p.ProtoIdx) >= h.ProtoIDsSize || p.NameIdx >= h.StringIDsSize {
				return perr("method_ids", c.Pos()-8, fmt.Errorf("%w: index out of range in method %d", ErrInvalid, i))
			}
		}
	}
	return c.Err()
}

func (f *File) parseClassDefs() error {
	h := &f.Header
	if err := f.checkTable("class_defs", h.ClassDefsOff, h.ClassDefsSize, 32); err != nil {
		return err
	}
	f.ClassDefs = make([]ClassDef, h.ClassDefsSize)
	c := newCursor(f.data, h.ClassDefsOff, "class_defs")
	for i := range f.ClassDefs {
		d := &f.ClassDefs[i]
		d.ClassIdx = c.u32("")
		d.AccessFlags = c.u32("")
		d.SuperclassIdx = c.u32("")
		d.InterfacesOff = c.u32("")
		d.SourceFileIdx = c.u32("")
		d.AnnotationsOff = c.u32("")
		d.ClassDataOff = c.u32("")
		d.StaticValuesOff = c.u32("")
		if c.Err() == nil {
			if d.ClassIdx >= h.TypeIDsSize {
				return perr("class_defs", c.Pos()-32, fmt.Errorf("%w: class_idx %d out of range in class_def %d", ErrInvalid, d.ClassIdx, i))
			}
			if d.SuperclassIdx != NoIndex && d.SuperclassIdx >= h.TypeIDsSize {
				return perr("class_defs", c.Pos()-32, fmt.Errorf("%w: superclass_idx %d out of range in class_def %d", ErrInvalid, d.SuperclassIdx, i))
			}
			if d.SourceFileIdx != NoIndex && d.SourceFileIdx >= h.StringIDsSize {
				return perr("class_defs", c.Pos()-32, fmt.Errorf("%w: source_file_idx out of range in class_def %d", ErrInvalid, i))
			}
		}
	}
	return c.Err()
}

func (f *File) parseMapList() {
	off := f.Header.MapOff
	if off == 0 {
		f.warn("map_off is zero", 0x34)
		return
	}
	c := newCursor(f.data, off, "map_list")
	size := c.u32("size")
	if c.Err() != nil || uint64(off)+4+uint64(size)*12 > uint64(len(f.data)) {
		f.warn("map_list unreadable or oversized", off)
		return
	}
	f.MapList = make([]MapItem, size)
	for i := range f.MapList {
		f.MapList[i].Type = c.u16("")
		c.u16("") // unused
		f.MapList[i].Size = c.u32("")
		f.MapList[i].Offset = c.u32("")
	}
	if c.Err() != nil {
		f.MapList = nil
		f.warn("map_list truncated", off)
		return
	}
	f.MapListRange = Range{off, 4 + size*12}
}

// mapItem returns the map_list entry of the given type, if present.
func (f *File) mapItem(t uint16) (MapItem, bool) {
	for _, m := range f.MapList {
		if m.Type == t {
			return m, true
		}
	}
	return MapItem{}, false
}

func (f *File) parseCallSitesAndHandles() error {
	if m, ok := f.mapItem(TypeCallSiteIDItem); ok && m.Size > 0 {
		if err := f.checkTable("call_site_ids", m.Offset, m.Size, 4); err != nil {
			return err
		}
		f.CallSiteIDs = make([]uint32, m.Size)
		c := newCursor(f.data, m.Offset, "call_site_ids")
		for i := range f.CallSiteIDs {
			f.CallSiteIDs[i] = c.u32("")
		}
		if c.Err() != nil {
			return c.Err()
		}
	}
	if m, ok := f.mapItem(TypeMethodHandleItem); ok && m.Size > 0 {
		if err := f.checkTable("method_handles", m.Offset, m.Size, 8); err != nil {
			return err
		}
		f.MethodHandles = make([]MethodHandle, m.Size)
		c := newCursor(f.data, m.Offset, "method_handles")
		for i := range f.MethodHandles {
			mh := &f.MethodHandles[i]
			mh.Type = c.u16("")
			c.u16("")
			mh.FieldOrMethodID = c.u16("")
			c.u16("")
			if c.Err() != nil {
				break
			}
			if mh.Type > MHInvokeInterface {
				return perr("method_handles", c.Pos()-8, fmt.Errorf("%w: method_handle_type %d in method handle %d", ErrInvalid, mh.Type, i))
			}
			limit := f.Header.MethodIDsSize
			if mh.IsFieldHandle() {
				limit = f.Header.FieldIDsSize
			}
			if uint32(mh.FieldOrMethodID) >= limit {
				return perr("method_handles", c.Pos()-8, fmt.Errorf("%w: field_or_method_id %d out of range in method handle %d", ErrInvalid, mh.FieldOrMethodID, i))
			}
		}
		if c.Err() != nil {
			return c.Err()
		}
	}
	return nil
}

// CallSiteIDsRange returns the byte range of the call_site_ids section.
func (f *File) CallSiteIDsRange() Range {
	if m, ok := f.mapItem(TypeCallSiteIDItem); ok {
		return Range{m.Offset, m.Size * 4}
	}
	return Range{}
}

// MethodHandlesRange returns the byte range of the method_handles section.
func (f *File) MethodHandlesRange() Range {
	if m, ok := f.mapItem(TypeMethodHandleItem); ok {
		return Range{m.Offset, m.Size * 8}
	}
	return Range{}
}

func (f *File) parseAllClassData() {
	f.classData = make([]*ClassData, len(f.ClassDefs))
	f.classDataErr = make([]error, len(f.ClassDefs))
	for i, d := range f.ClassDefs {
		if d.ClassDataOff == 0 {
			continue
		}
		cd, err := f.parseClassData(d.ClassDataOff, nil)
		if err != nil {
			f.classDataErr[i] = err
			f.warn(fmt.Sprintf("class_data for class_def %d: %v", i, err), d.ClassDataOff)
			continue
		}
		f.classData[i] = cd
	}
}

// ParseClassData parses the class_data_item at the given offset, optionally
// recording field views.
func (f *File) parseClassData(off uint32, rec *[]FieldView) (*ClassData, error) {
	c := newCursor(f.data, off, "class_data_item")
	c.rec = rec
	cd := &ClassData{}
	ns := c.uleb("static_fields_size")
	ni := c.uleb("instance_fields_size")
	nd := c.uleb("direct_methods_size")
	nv := c.uleb("virtual_methods_size")
	if c.Err() != nil {
		return nil, c.Err()
	}
	// Each entry needs at least 2 (fields) or 3 (methods) bytes; reject
	// absurd counts before allocating.
	need := uint64(ns)*2 + uint64(ni)*2 + uint64(nd)*3 + uint64(nv)*3
	if need > uint64(c.remaining()) {
		return nil, perr("class_data_item", off, fmt.Errorf("%w: member counts %d/%d/%d/%d", ErrTooLarge, ns, ni, nd, nv))
	}
	// Member indices are encoded as differences from the previous entry (the
	// first is absolute). Accumulate in 64 bits so a hostile difference cannot
	// wrap around into a valid-looking index; a non-increasing index is kept
	// (the runtime rejects such files) but recorded as a warning.
	readFields := func(n uint32, label string) ([]EncodedField, error) {
		out := make([]EncodedField, 0, n)
		var idx uint64
		for i := uint32(0); i < n; i++ {
			start := c.Pos()
			var nDiff, nFlags string
			if rec != nil {
				nDiff = fmt.Sprintf("%s[%d].field_idx_diff", label, i)
				nFlags = fmt.Sprintf("%s[%d].access_flags", label, i)
			}
			diff := c.uleb(nDiff)
			flags := c.uleb(nFlags)
			if c.Err() != nil {
				return nil, c.Err()
			}
			if i > 0 && diff == 0 {
				f.warn(fmt.Sprintf("class_data_item %s[%d] repeats the previous field_idx", label, i), start)
			}
			idx += uint64(diff)
			if idx >= uint64(f.Header.FieldIDsSize) {
				return nil, perr("class_data_item", start, fmt.Errorf("%w: field_idx %d out of range", ErrInvalid, idx))
			}
			out = append(out, EncodedField{FieldIdx: uint32(idx), AccessFlags: flags, Range: Range{start, c.Pos() - start}})
		}
		return out, nil
	}
	readMethods := func(n uint32, label string) ([]EncodedMethod, error) {
		out := make([]EncodedMethod, 0, n)
		var idx uint64
		for i := uint32(0); i < n; i++ {
			start := c.Pos()
			var nDiff, nFlags, nCode string
			if rec != nil {
				nDiff = fmt.Sprintf("%s[%d].method_idx_diff", label, i)
				nFlags = fmt.Sprintf("%s[%d].access_flags", label, i)
				nCode = fmt.Sprintf("%s[%d].code_off", label, i)
			}
			diff := c.uleb(nDiff)
			flags := c.uleb(nFlags)
			codeOff := c.uleb(nCode)
			if c.Err() != nil {
				return nil, c.Err()
			}
			if i > 0 && diff == 0 {
				f.warn(fmt.Sprintf("class_data_item %s[%d] repeats the previous method_idx", label, i), start)
			}
			idx += uint64(diff)
			if idx >= uint64(f.Header.MethodIDsSize) {
				return nil, perr("class_data_item", start, fmt.Errorf("%w: method_idx %d out of range", ErrInvalid, idx))
			}
			if codeOff != 0 && uint64(codeOff)+16 > uint64(len(f.data)) {
				return nil, perr("class_data_item", start, fmt.Errorf("%w: code_off 0x%x", ErrOutOfBounds, codeOff))
			}
			out = append(out, EncodedMethod{MethodIdx: uint32(idx), AccessFlags: flags, CodeOff: codeOff, Range: Range{start, c.Pos() - start}})
		}
		return out, nil
	}
	var err error
	if cd.StaticFields, err = readFields(ns, "static_fields"); err != nil {
		return nil, err
	}
	if cd.InstanceFields, err = readFields(ni, "instance_fields"); err != nil {
		return nil, err
	}
	if cd.DirectMethods, err = readMethods(nd, "direct_methods"); err != nil {
		return nil, err
	}
	if cd.VirtualMethods, err = readMethods(nv, "virtual_methods"); err != nil {
		return nil, err
	}
	cd.Range = Range{off, c.Pos() - off}
	return cd, nil
}

// ClassData returns the parsed class_data_item for a class def, or nil.
func (f *File) ClassData(classDefIdx uint32) *ClassData {
	if int(classDefIdx) >= len(f.classData) {
		return nil
	}
	return f.classData[classDefIdx]
}

// ClassDataError returns the parse error for a class def's class_data, if any.
func (f *File) ClassDataError(classDefIdx uint32) error {
	if int(classDefIdx) >= len(f.classDataErr) {
		return nil
	}
	return f.classDataErr[classDefIdx]
}

// ClassDataFields re-parses a class_data_item recording field views.
func (f *File) ClassDataFields(off uint32) ([]FieldView, *ClassData, error) {
	var views []FieldView
	cd, err := f.parseClassData(off, &views)
	return views, cd, err
}

// TypeList parses the type_list at the given offset (0 yields an empty list).
func (f *File) TypeList(off uint32) (*TypeList, error) {
	if off == 0 {
		return &TypeList{}, nil
	}
	c := newCursor(f.data, off, "type_list")
	size := c.u32("size")
	if c.Err() != nil {
		return nil, c.Err()
	}
	if uint64(size)*2 > uint64(c.remaining()) {
		return nil, perr("type_list", off, fmt.Errorf("%w: %d entries", ErrTooLarge, size))
	}
	tl := &TypeList{Types: make([]uint16, size)}
	for i := range tl.Types {
		tl.Types[i] = c.u16("")
		if uint32(tl.Types[i]) >= f.Header.TypeIDsSize {
			return nil, perr("type_list", c.Pos()-2, fmt.Errorf("%w: type_idx %d", ErrInvalid, tl.Types[i]))
		}
	}
	tl.Range = Range{off, 4 + size*2}
	return tl, c.Err()
}

// ---- string / descriptor accessors ----

// StringCount returns the number of strings.
func (f *File) StringCount() int { return len(f.strings) }

// String returns the decoded string at idx ("" if out of range).
func (f *File) String(idx uint32) string {
	if int(idx) >= len(f.strings) {
		return ""
	}
	return f.strings[idx].str
}

// StringOK returns the string and whether idx is valid.
func (f *File) StringOK(idx uint32) (string, bool) {
	if int(idx) >= len(f.strings) {
		return "", false
	}
	return f.strings[idx].str, true
}

// StringDataRange returns the byte range of string_data_item idx.
func (f *File) StringDataRange(idx uint32) (Range, bool) {
	if int(idx) >= len(f.strings) {
		return Range{}, false
	}
	it := &f.strings[idx]
	return Range{it.dataOff, it.dataLen}, true
}

// StringInfo returns metadata about a string entry.
func (f *File) StringInfo(idx uint32) (utf16Size uint32, malformed bool, sizeMismatch bool, ok bool) {
	if int(idx) >= len(f.strings) {
		return 0, false, false, false
	}
	it := &f.strings[idx]
	return it.utf16Size, it.flags&1 != 0, it.flags&2 != 0, true
}

// TypeDesc returns the descriptor for a type index.
func (f *File) TypeDesc(idx uint32) string {
	if int(idx) >= len(f.TypeIDs) {
		return ""
	}
	return f.String(f.TypeIDs[idx])
}

// TypeDescOK returns the descriptor and validity of a type index.
func (f *File) TypeDescOK(idx uint32) (string, bool) {
	if int(idx) >= len(f.TypeIDs) {
		return "", false
	}
	return f.String(f.TypeIDs[idx]), true
}

// ProtoParams returns the parameter type descriptors for a proto.
func (f *File) ProtoParams(idx uint32) ([]string, error) {
	if int(idx) >= len(f.ProtoIDs) {
		return nil, fmt.Errorf("proto index %d out of range", idx)
	}
	tl, err := f.TypeList(f.ProtoIDs[idx].ParametersOff)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(tl.Types))
	for i, t := range tl.Types {
		out[i] = f.TypeDesc(uint32(t))
	}
	return out, nil
}

// ProtoDesc returns the "(params)ret" descriptor for a proto.
func (f *File) ProtoDesc(idx uint32) string {
	if int(idx) >= len(f.ProtoIDs) {
		return ""
	}
	params, err := f.ProtoParams(idx)
	var sb []byte
	sb = append(sb, '(')
	if err == nil {
		for _, p := range params {
			sb = append(sb, p...)
		}
	} else {
		sb = append(sb, "?"...)
	}
	sb = append(sb, ')')
	sb = append(sb, f.TypeDesc(f.ProtoIDs[idx].ReturnTypeIdx)...)
	return string(sb)
}

// MethodDesc returns the canonical "Lcls;->name(params)ret" descriptor.
func (f *File) MethodDesc(idx uint32) string {
	if int(idx) >= len(f.MethodIDs) {
		return ""
	}
	m := f.MethodIDs[idx]
	return f.TypeDesc(uint32(m.ClassIdx)) + "->" + f.String(m.NameIdx) + f.ProtoDesc(uint32(m.ProtoIdx))
}

// MethodShortDesc returns "name(params)ret" without the class.
func (f *File) MethodShortDesc(idx uint32) string {
	if int(idx) >= len(f.MethodIDs) {
		return ""
	}
	m := f.MethodIDs[idx]
	return f.String(m.NameIdx) + f.ProtoDesc(uint32(m.ProtoIdx))
}

// FieldDesc returns the canonical "Lcls;->name:Ltype;" descriptor.
func (f *File) FieldDesc(idx uint32) string {
	if int(idx) >= len(f.FieldIDs) {
		return ""
	}
	fd := f.FieldIDs[idx]
	return f.TypeDesc(uint32(fd.ClassIdx)) + "->" + f.String(fd.NameIdx) + ":" + f.TypeDesc(uint32(fd.TypeIdx))
}

// FieldShortDesc returns "name:type".
func (f *File) FieldShortDesc(idx uint32) string {
	if int(idx) >= len(f.FieldIDs) {
		return ""
	}
	fd := f.FieldIDs[idx]
	return f.String(fd.NameIdx) + ":" + f.TypeDesc(uint32(fd.TypeIdx))
}

func (f *File) buildLookups() {
	f.once.Do(func() {
		f.typeByDesc = make(map[string]uint32, len(f.TypeIDs))
		for i := range f.TypeIDs {
			f.typeByDesc[f.TypeDesc(uint32(i))] = uint32(i)
		}
		f.classByType = make(map[uint32]uint32, len(f.ClassDefs))
		for i, d := range f.ClassDefs {
			if _, dup := f.classByType[d.ClassIdx]; dup {
				f.warn(fmt.Sprintf("duplicate class_def for type %s", f.TypeDesc(d.ClassIdx)), f.Header.ClassDefsOff+uint32(i)*32)
				continue
			}
			f.classByType[d.ClassIdx] = uint32(i)
		}
		f.stringByVal = make(map[string]uint32, len(f.strings))
		for i := range f.strings {
			f.stringByVal[f.strings[i].str] = uint32(i)
		}
	})
}

// TypeIndex looks up a type by descriptor.
func (f *File) TypeIndex(desc string) (uint32, bool) {
	f.buildLookups()
	i, ok := f.typeByDesc[desc]
	return i, ok
}

// ClassDefIndexByType returns the class_def index defining the type idx.
func (f *File) ClassDefIndexByType(typeIdx uint32) (uint32, bool) {
	f.buildLookups()
	i, ok := f.classByType[typeIdx]
	return i, ok
}

// ClassDefIndex returns the class_def index for a class descriptor.
func (f *File) ClassDefIndex(desc string) (uint32, bool) {
	t, ok := f.TypeIndex(desc)
	if !ok {
		return 0, false
	}
	return f.ClassDefIndexByType(t)
}

// StringIndex looks up a string value.
func (f *File) StringIndex(s string) (uint32, bool) {
	f.buildLookups()
	i, ok := f.stringByVal[s]
	return i, ok
}

// ---- fixed-size table item ranges ----

// StringIDRange returns the range of string_id_item idx.
func (f *File) StringIDRange(idx uint32) Range { return Range{f.Header.StringIDsOff + idx*4, 4} }

// TypeIDRange returns the range of type_id_item idx.
func (f *File) TypeIDRange(idx uint32) Range { return Range{f.Header.TypeIDsOff + idx*4, 4} }

// ProtoIDRange returns the range of proto_id_item idx.
func (f *File) ProtoIDRange(idx uint32) Range { return Range{f.Header.ProtoIDsOff + idx*12, 12} }

// FieldIDRange returns the range of field_id_item idx.
func (f *File) FieldIDRange(idx uint32) Range { return Range{f.Header.FieldIDsOff + idx*8, 8} }

// MethodIDRange returns the range of method_id_item idx.
func (f *File) MethodIDRange(idx uint32) Range { return Range{f.Header.MethodIDsOff + idx*8, 8} }

// ClassDefRange returns the range of class_def_item idx.
func (f *File) ClassDefRange(idx uint32) Range { return Range{f.Header.ClassDefsOff + idx*32, 32} }

// CallSiteIDRange returns the range of call_site_id_item idx.
func (f *File) CallSiteIDRange(idx uint32) Range { return Range{f.CallSiteIDsRange().Off + idx*4, 4} }

// MethodHandleRange returns the range of method_handle_item idx.
func (f *File) MethodHandleRange(idx uint32) Range {
	return Range{f.MethodHandlesRange().Off + idx*8, 8}
}

// Map item type constants.
const (
	TypeHeaderItem               = 0x0000
	TypeStringIDItem             = 0x0001
	TypeTypeIDItem               = 0x0002
	TypeProtoIDItem              = 0x0003
	TypeFieldIDItem              = 0x0004
	TypeMethodIDItem             = 0x0005
	TypeClassDefItem             = 0x0006
	TypeCallSiteIDItem           = 0x0007
	TypeMethodHandleItem         = 0x0008
	TypeMapList                  = 0x1000
	TypeTypeList                 = 0x1001
	TypeAnnotationSetRefList     = 0x1002
	TypeAnnotationSetItem        = 0x1003
	TypeClassDataItem            = 0x2000
	TypeCodeItem                 = 0x2001
	TypeStringDataItem           = 0x2002
	TypeDebugInfoItem            = 0x2003
	TypeAnnotationItem           = 0x2004
	TypeEncodedArrayItem         = 0x2005
	TypeAnnotationsDirectoryItem = 0x2006
	TypeHiddenAPIClassDataItem   = 0xF000
)

// MapItemTypeName returns the spec name for a map item type.
func MapItemTypeName(t uint16) string {
	switch t {
	case TypeHeaderItem:
		return "header_item"
	case TypeStringIDItem:
		return "string_id_item"
	case TypeTypeIDItem:
		return "type_id_item"
	case TypeProtoIDItem:
		return "proto_id_item"
	case TypeFieldIDItem:
		return "field_id_item"
	case TypeMethodIDItem:
		return "method_id_item"
	case TypeClassDefItem:
		return "class_def_item"
	case TypeCallSiteIDItem:
		return "call_site_id_item"
	case TypeMethodHandleItem:
		return "method_handle_item"
	case TypeMapList:
		return "map_list"
	case TypeTypeList:
		return "type_list"
	case TypeAnnotationSetRefList:
		return "annotation_set_ref_list"
	case TypeAnnotationSetItem:
		return "annotation_set_item"
	case TypeClassDataItem:
		return "class_data_item"
	case TypeCodeItem:
		return "code_item"
	case TypeStringDataItem:
		return "string_data_item"
	case TypeDebugInfoItem:
		return "debug_info_item"
	case TypeAnnotationItem:
		return "annotation_item"
	case TypeEncodedArrayItem:
		return "encoded_array_item"
	case TypeAnnotationsDirectoryItem:
		return "annotations_directory_item"
	case TypeHiddenAPIClassDataItem:
		return "hiddenapi_class_data_item"
	}
	return fmt.Sprintf("unknown_0x%04x", t)
}

// Access flag constants.
const (
	AccPublic               = 0x1
	AccPrivate              = 0x2
	AccProtected            = 0x4
	AccStatic               = 0x8
	AccFinal                = 0x10
	AccSynchronized         = 0x20
	AccVolatile             = 0x40
	AccBridge               = 0x40
	AccTransient            = 0x80
	AccVarargs              = 0x80
	AccNative               = 0x100
	AccInterface            = 0x200
	AccAbstract             = 0x400
	AccStrict               = 0x800
	AccSynthetic            = 0x1000
	AccAnnotation           = 0x2000
	AccEnum                 = 0x4000
	AccConstructor          = 0x10000
	AccDeclaredSynchronized = 0x20000
)

// AccessFlagNames renders access flags for a class, field, or method.
func AccessFlagNames(flags uint32, kind string) []string {
	var out []string
	add := func(bit uint32, name string) {
		if flags&bit != 0 {
			out = append(out, name)
		}
	}
	add(AccPublic, "public")
	add(AccPrivate, "private")
	add(AccProtected, "protected")
	add(AccStatic, "static")
	add(AccFinal, "final")
	switch kind {
	case "field":
		add(AccVolatile, "volatile")
		add(AccTransient, "transient")
	case "method":
		add(AccSynchronized, "synchronized")
		add(AccBridge, "bridge")
		add(AccVarargs, "varargs")
		add(AccNative, "native")
		add(AccAbstract, "abstract")
		add(AccStrict, "strictfp")
		add(AccConstructor, "constructor")
		add(AccDeclaredSynchronized, "declared-synchronized")
	default:
		add(AccInterface, "interface")
		add(AccAbstract, "abstract")
	}
	add(AccSynthetic, "synthetic")
	add(AccAnnotation, "annotation")
	add(AccEnum, "enum")
	return out
}
