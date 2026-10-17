package dexfile

import "fmt"

// Hidden API restriction flags (DEX 039+, hiddenapi_class_data_item).
const (
	HiddenAPIWhitelist    = 0
	HiddenAPIGreylist     = 1
	HiddenAPIBlacklist    = 2
	HiddenAPIGreylistMaxO = 3
	HiddenAPIGreylistMaxP = 4
	HiddenAPIGreylistMaxQ = 5
	HiddenAPIGreylistMaxR = 6
)

// HiddenAPIFlagName renders a hiddenapi restriction value.
func HiddenAPIFlagName(v uint32) string {
	names := []string{"whitelist", "greylist", "blacklist", "greylist-max-o", "greylist-max-p", "greylist-max-q", "greylist-max-r"}
	if int(v) < len(names) {
		return names[v]
	}
	return fmt.Sprintf("unknown_%d", v)
}

type hiddenAPIData struct {
	rng      Range
	perClass map[uint32]*HiddenAPIClass // class def idx -> flags
}

// HiddenAPIClass holds the restriction flags for the members of one class, in
// class_data order (static fields, instance fields, direct methods, virtual methods).
type HiddenAPIClass struct {
	Range          Range
	StaticFields   []uint32
	InstanceFields []uint32
	DirectMethods  []uint32
	VirtualMethods []uint32
}

func (f *File) parseHiddenAPI() {
	m, ok := f.mapItem(TypeHiddenAPIClassDataItem)
	if !ok || m.Size == 0 {
		return
	}
	off := m.Offset
	c := newCursor(f.data, off, "hiddenapi_class_data_item")
	size := c.u32("size")
	if c.Err() != nil || uint64(off)+uint64(size) > uint64(len(f.data)) {
		f.warn("hiddenapi_class_data_item unreadable", off)
		return
	}
	n := uint32(len(f.ClassDefs))
	if uint64(n)*4+4 > uint64(size) {
		f.warn("hiddenapi_class_data_item too small for class_defs", off)
		return
	}
	hd := &hiddenAPIData{rng: Range{off, size}, perClass: map[uint32]*HiddenAPIClass{}}
	offsets := make([]uint32, n)
	for i := range offsets {
		offsets[i] = c.u32("")
	}
	if c.Err() != nil {
		f.warn("hiddenapi offsets truncated", off)
		return
	}
	for i, rel := range offsets {
		if rel == 0 {
			continue
		}
		cd := f.classData[i]
		if cd == nil {
			continue
		}
		if uint64(rel) >= uint64(size) {
			f.warn(fmt.Sprintf("hiddenapi offset for class %d out of item", i), off)
			continue
		}
		hc := &HiddenAPIClass{}
		fc := newCursor(f.data, off+rel, "hiddenapi_flags")
		read := func(n int) []uint32 {
			out := make([]uint32, n)
			for j := range out {
				out[j] = fc.uleb("")
			}
			return out
		}
		hc.StaticFields = read(len(cd.StaticFields))
		hc.InstanceFields = read(len(cd.InstanceFields))
		hc.DirectMethods = read(len(cd.DirectMethods))
		hc.VirtualMethods = read(len(cd.VirtualMethods))
		if fc.Err() != nil {
			f.warn(fmt.Sprintf("hiddenapi flags for class %d truncated", i), off+rel)
			continue
		}
		hc.Range = Range{off + rel, fc.Pos() - (off + rel)}
		hd.perClass[uint32(i)] = hc
	}
	f.hiddenAPI = hd
}

// HiddenAPI returns the hidden-API flags for a class def, if present.
func (f *File) HiddenAPI(classDefIdx uint32) (*HiddenAPIClass, bool) {
	if f.hiddenAPI == nil {
		return nil, false
	}
	h, ok := f.hiddenAPI.perClass[classDefIdx]
	return h, ok
}

// HiddenAPIRange returns the range of the hiddenapi_class_data_item, if any.
func (f *File) HiddenAPIRange() (Range, bool) {
	if f.hiddenAPI == nil {
		return Range{}, false
	}
	return f.hiddenAPI.rng, true
}
