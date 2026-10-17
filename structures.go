package dexfile

import (
	"sort"
)

// Sub-structure kinds that are not map item types. They use values outside
// the map item type space.
const (
	KindInsns               uint16 = 0xE001 // instruction array of a code_item
	KindTries               uint16 = 0xE002 // try_item array of a code_item
	KindCatchHandlerList    uint16 = 0xE003 // encoded_catch_handler_list of a code_item
	KindInstruction         uint16 = 0xE004 // one instruction (computed on demand)
	KindEncodedField        uint16 = 0xE005 // one encoded_field (computed on demand)
	KindEncodedMethod       uint16 = 0xE006 // one encoded_method (computed on demand)
	KindTryItem             uint16 = 0xE007
	KindEncodedCatchHandler uint16 = 0xE008
	KindSection             uint16 = 0xE009 // header-described table section
)

// StructKindName renders a structure kind (map item type or sub-kind).
func StructKindName(t uint16) string {
	switch t {
	case KindInsns:
		return "insns"
	case KindTries:
		return "tries"
	case KindCatchHandlerList:
		return "encoded_catch_handler_list"
	case KindInstruction:
		return "instruction"
	case KindEncodedField:
		return "encoded_field"
	case KindEncodedMethod:
		return "encoded_method"
	case KindTryItem:
		return "try_item"
	case KindEncodedCatchHandler:
		return "encoded_catch_handler"
	case KindSection:
		return "section"
	}
	return MapItemTypeName(t)
}

// StructRef locates one structure instance in the file.
type StructRef struct {
	Kind  uint16 // map item type or Kind* sub-kind
	Range Range
	// Index identifies the instance: table index for *_id_item and class_def,
	// string index for string_data_item, class def index for class-owned
	// items, method index for code_item/debug_info_item, call site index for
	// call-site encoded arrays; NoIndex when not applicable.
	Index uint32
	// Owner is the class def index that owns the item when known, else NoIndex.
	Owner uint32
	Depth uint8 // 0 = section, 1 = item, 2 = sub-item
}

func (f *File) buildStructures() {
	f.structsOnce.Do(func() {
		b := &structBuilder{f: f, seen: map[uint64]bool{}}
		b.build()
		sort.Slice(b.out, func(i, j int) bool {
			if b.out[i].Depth != b.out[j].Depth {
				return b.out[i].Depth < b.out[j].Depth
			}
			return b.out[i].Range.Off < b.out[j].Range.Off
		})
		f.structs = b.out
		// Precompute depth segment boundaries for O(log n) lookups.
		for d := 0; d < len(f.depthStart); d++ {
			f.depthStart[d] = sort.Search(len(f.structs), func(i int) bool { return int(f.structs[i].Depth) >= d })
		}
	})
}

// fixedItem synthesises the fixed-size table item containing off inside a
// section, so that per-item entries need not be stored in the registry.
func (f *File) fixedItem(sec StructRef, off uint32) (StructRef, bool) {
	var size uint32
	switch uint16(sec.Index) {
	case TypeStringIDItem, TypeTypeIDItem, TypeCallSiteIDItem:
		size = 4
	case TypeProtoIDItem:
		size = 12
	case TypeFieldIDItem, TypeMethodIDItem, TypeMethodHandleItem:
		size = 8
	case TypeClassDefItem:
		size = 32
	default:
		return StructRef{}, false
	}
	idx := (off - sec.Range.Off) / size
	r := StructRef{Kind: uint16(sec.Index), Range: Range{sec.Range.Off + idx*size, size}, Index: idx, Owner: NoIndex, Depth: 1}
	if uint16(sec.Index) == TypeClassDefItem {
		r.Owner = idx
	}
	return r, true
}

type structBuilder struct {
	f    *File
	out  []StructRef
	seen map[uint64]bool // kind<<32|off
}

func (b *structBuilder) add(kind uint16, r Range, index, owner uint32, depth uint8) bool {
	if r.Len == 0 {
		return false
	}
	key := uint64(kind)<<32 | uint64(r.Off)
	if b.seen[key] {
		return false
	}
	b.seen[key] = true
	b.out = append(b.out, StructRef{Kind: kind, Range: r, Index: index, Owner: owner, Depth: depth})
	return true
}

func (b *structBuilder) build() {
	f := b.f
	h := &f.Header
	// Sections (depth 0): header and fixed-size tables.
	b.add(TypeHeaderItem, f.HeaderRange(), NoIndex, NoIndex, 0)
	b.add(KindSection, Range{h.StringIDsOff, h.StringIDsSize * 4}, TypeStringIDItem, NoIndex, 0)
	b.add(KindSection, Range{h.TypeIDsOff, h.TypeIDsSize * 4}, TypeTypeIDItem, NoIndex, 0)
	b.add(KindSection, Range{h.ProtoIDsOff, h.ProtoIDsSize * 12}, TypeProtoIDItem, NoIndex, 0)
	b.add(KindSection, Range{h.FieldIDsOff, h.FieldIDsSize * 8}, TypeFieldIDItem, NoIndex, 0)
	b.add(KindSection, Range{h.MethodIDsOff, h.MethodIDsSize * 8}, TypeMethodIDItem, NoIndex, 0)
	b.add(KindSection, Range{h.ClassDefsOff, h.ClassDefsSize * 32}, TypeClassDefItem, NoIndex, 0)
	b.add(KindSection, f.CallSiteIDsRange(), TypeCallSiteIDItem, NoIndex, 0)
	b.add(KindSection, f.MethodHandlesRange(), TypeMethodHandleItem, NoIndex, 0)
	if h.DataSize > 0 && uint64(h.DataOff)+uint64(h.DataSize) <= uint64(len(f.data)) {
		b.add(KindSection, Range{h.DataOff, h.DataSize}, 0xFFFF, NoIndex, 0)
	}
	if h.LinkSize > 0 && uint64(h.LinkOff)+uint64(h.LinkSize) <= uint64(len(f.data)) {
		b.add(KindSection, Range{h.LinkOff, h.LinkSize}, 0xFFFE, NoIndex, 0)
	}

	// Items (depth 1).
	// Fixed-size table items (string/type/proto/field/method ids, class_defs,
	// call sites, method handles) are synthesised on demand from their section
	// by fixedItem; only variable-length items are registered.
	for i := range f.StringIDs {
		if r, ok := f.StringDataRange(uint32(i)); ok {
			b.add(TypeStringDataItem, r, uint32(i), NoIndex, 1)
		}
	}
	for i, p := range f.ProtoIDs {
		if p.ParametersOff != 0 {
			if tl, err := f.TypeList(p.ParametersOff); err == nil {
				b.add(TypeTypeList, tl.Range, uint32(i), NoIndex, 1)
			}
		}
	}
	for i := range f.CallSiteIDs {
		if ea, err := f.EncodedArrayItem(f.CallSiteIDs[i]); err == nil {
			b.add(TypeEncodedArrayItem, ea.Range, uint32(i), NoIndex, 1)
		}
	}
	if f.MapListRange.Len > 0 {
		b.add(TypeMapList, f.MapListRange, NoIndex, NoIndex, 1)
	}
	if r, ok := f.HiddenAPIRange(); ok {
		b.add(TypeHiddenAPIClassDataItem, r, NoIndex, NoIndex, 1)
	}
	for i, d := range f.ClassDefs {
		ci := uint32(i)
		if d.InterfacesOff != 0 {
			if tl, err := f.TypeList(d.InterfacesOff); err == nil {
				b.add(TypeTypeList, tl.Range, NoIndex, ci, 1)
			}
		}
		if d.StaticValuesOff != 0 {
			if ea, err := f.EncodedArrayItem(d.StaticValuesOff); err == nil {
				b.add(TypeEncodedArrayItem, ea.Range, NoIndex, ci, 1)
			}
		}
		if cd := f.classData[i]; cd != nil {
			b.add(TypeClassDataItem, cd.Range, ci, ci, 1)
			for _, lists := range [][]EncodedMethod{cd.DirectMethods, cd.VirtualMethods} {
				for _, m := range lists {
					if m.CodeOff == 0 {
						continue
					}
					code, err := f.Code(m.CodeOff)
					if err != nil {
						continue
					}
					if b.add(TypeCodeItem, code.Range, m.MethodIdx, ci, 1) {
						b.add(KindInsns, code.InsnsRange, m.MethodIdx, ci, 2)
						b.add(KindTries, code.TriesRange, m.MethodIdx, ci, 2)
						b.add(KindCatchHandlerList, code.HandlersRange, m.MethodIdx, ci, 2)
					}
					if code.DebugInfoOff != 0 {
						if di, err := f.DebugInfo(code.DebugInfoOff, code.InsnsSize); err == nil {
							b.add(TypeDebugInfoItem, di.Range, m.MethodIdx, ci, 1)
						}
					}
				}
			}
		}
		if d.AnnotationsOff != 0 {
			b.addAnnotations(d.AnnotationsOff, ci)
		}
	}
}

func (b *structBuilder) addAnnotationSet(off uint32, owner uint32) {
	f := b.f
	as, err := f.AnnotationSet(off)
	if err != nil {
		return
	}
	if !b.add(TypeAnnotationSetItem, as.Range, NoIndex, owner, 1) {
		return
	}
	for _, o := range as.Offsets {
		if ai, err := f.AnnotationItem(o); err == nil {
			b.add(TypeAnnotationItem, ai.Range, NoIndex, owner, 1)
		}
	}
}

func (b *structBuilder) addAnnotations(off uint32, owner uint32) {
	f := b.f
	dir, err := f.AnnotationsDirectory(off)
	if err != nil {
		return
	}
	if !b.add(TypeAnnotationsDirectoryItem, dir.Range, owner, owner, 1) {
		return
	}
	if dir.ClassAnnotationsOff != 0 {
		b.addAnnotationSet(dir.ClassAnnotationsOff, owner)
	}
	for _, fa := range dir.FieldAnnotations {
		b.addAnnotationSet(fa.AnnotationsOff, owner)
	}
	for _, ma := range dir.MethodAnnotations {
		b.addAnnotationSet(ma.AnnotationsOff, owner)
	}
	for _, pa := range dir.ParameterAnnotations {
		if l, err := f.AnnotationSetRefList(pa.AnnotationsOff); err == nil {
			if b.add(TypeAnnotationSetRefList, l.Range, pa.MethodIdx, owner, 1) {
				for _, o := range l.Offsets {
					if o != 0 {
						b.addAnnotationSet(o, owner)
					}
				}
			}
		}
	}
}

// Structures returns the complete structure registry, sorted by depth then
// offset. It is built lazily on first use.
func (f *File) Structures() []StructRef {
	f.buildStructures()
	return f.structs
}

// StructuresAt returns every registered structure whose range contains the
// offset, outermost first. Instruction and encoded-member granularity is
// added on demand when the offset falls inside an insns region or a
// class_data_item.
func (f *File) StructuresAt(off uint32) []StructRef {
	f.buildStructures()
	var out []StructRef
	s := f.structs
	// Depth 0 entries are few: linear scan.
	for i := 0; i < f.depthStart[1]; i++ {
		if s[i].Range.Contains(off) {
			out = append(out, s[i])
			if s[i].Kind == KindSection {
				if item, ok := f.fixedItem(s[i], off); ok {
					out = append(out, item)
				}
			}
		}
	}
	for depth := uint8(1); depth <= 2; depth++ {
		seg := s[f.depthStart[depth]:f.depthStart[depth+1]]
		// Items at one depth do not overlap; find the last with Off <= off.
		k := sort.Search(len(seg), func(j int) bool { return seg[j].Range.Off > off })
		for j := k - 1; j >= 0 && j >= k-4; j-- { // tolerate a few zero-length / shared edge cases
			if seg[j].Range.Contains(off) {
				out = append(out, seg[j])
				break
			}
		}
	}
	// Finer granularity.
	for _, r := range out {
		switch r.Kind {
		case KindInsns:
			if code, err := f.Code(r.Range.Off - 16); err == nil {
				recs, _ := code.InsnRecs(f)
				cu := (off - code.InsnsRange.Off) / 2
				if k := FindInsn(recs, cu); k >= 0 {
					rec := recs[k]
					out = append(out, StructRef{Kind: KindInstruction, Range: Range{code.InsnFileOffset(rec.Off), uint32(rec.Units) * 2}, Index: rec.Off, Owner: r.Index, Depth: 3})
				}
			}
		case KindTries:
			idx := (off - r.Range.Off) / 8
			out = append(out, StructRef{Kind: KindTryItem, Range: Range{r.Range.Off + idx*8, 8}, Index: idx, Owner: r.Index, Depth: 3})
		case KindCatchHandlerList:
			if code := f.codeByHandlersRange(r.Range); code != nil {
				for j, h := range code.Handlers {
					if h.Range.Contains(off) {
						out = append(out, StructRef{Kind: KindEncodedCatchHandler, Range: h.Range, Index: uint32(j), Owner: r.Index, Depth: 3})
					}
				}
			}
		case TypeClassDataItem:
			if cd := f.classData[r.Index]; cd != nil {
				for _, fl := range cd.StaticFields {
					if fl.Range.Contains(off) {
						out = append(out, StructRef{Kind: KindEncodedField, Range: fl.Range, Index: fl.FieldIdx, Owner: r.Index, Depth: 2})
					}
				}
				for _, fl := range cd.InstanceFields {
					if fl.Range.Contains(off) {
						out = append(out, StructRef{Kind: KindEncodedField, Range: fl.Range, Index: fl.FieldIdx, Owner: r.Index, Depth: 2})
					}
				}
				for _, m := range cd.DirectMethods {
					if m.Range.Contains(off) {
						out = append(out, StructRef{Kind: KindEncodedMethod, Range: m.Range, Index: m.MethodIdx, Owner: r.Index, Depth: 2})
					}
				}
				for _, m := range cd.VirtualMethods {
					if m.Range.Contains(off) {
						out = append(out, StructRef{Kind: KindEncodedMethod, Range: m.Range, Index: m.MethodIdx, Owner: r.Index, Depth: 2})
					}
				}
			}
		}
	}
	return out
}

// StructuresOverlapping returns registered structures of the given kind whose
// range intersects r, in offset order.
func (f *File) StructuresOverlapping(r Range, kind uint16) []StructRef {
	f.buildStructures()
	s := f.structs
	// Locate the depth segment for the kind (sub-kinds live at depth 2, items at depth 1, sections at 0).
	var depth uint8 = 1
	switch kind {
	case KindInsns, KindTries, KindCatchHandlerList:
		depth = 2
	case KindSection, TypeHeaderItem:
		depth = 0
	}
	seg := s[f.depthStart[depth]:f.depthStart[depth+1]]
	// First entry whose end is beyond r.Off: entries at one depth do not
	// overlap, so a binary search on Off is sufficient (back up one).
	k := sort.Search(len(seg), func(j int) bool { return seg[j].Range.Off >= r.Off })
	if k > 0 {
		k--
	}
	var out []StructRef
	for j := k; j < len(seg) && seg[j].Range.Off < r.End(); j++ {
		if seg[j].Kind == kind && seg[j].Range.End() > r.Off {
			out = append(out, seg[j])
		}
	}
	return out
}

func (f *File) codeByHandlersRange(r Range) *CodeItem {
	f.codeMu.Lock()
	defer f.codeMu.Unlock()
	for _, ci := range f.codeCache {
		if ci.HandlersRange == r {
			return ci
		}
	}
	return nil
}
