package dexfile

import (
	"fmt"
	"math"
)

// Encoded value types.
const (
	ValueByte         = 0x00
	ValueShort        = 0x02
	ValueChar         = 0x03
	ValueInt          = 0x04
	ValueLong         = 0x06
	ValueFloat        = 0x10
	ValueDouble       = 0x11
	ValueMethodType   = 0x15
	ValueMethodHandle = 0x16
	ValueString       = 0x17
	ValueType         = 0x18
	ValueField        = 0x19
	ValueMethod       = 0x1a
	ValueEnum         = 0x1b
	ValueArray        = 0x1c
	ValueAnnotation   = 0x1d
	ValueNull         = 0x1e
	ValueBoolean      = 0x1f
)

// ValueTypeName returns the spec name for an encoded value type.
func ValueTypeName(t uint8) string {
	switch t {
	case ValueByte:
		return "VALUE_BYTE"
	case ValueShort:
		return "VALUE_SHORT"
	case ValueChar:
		return "VALUE_CHAR"
	case ValueInt:
		return "VALUE_INT"
	case ValueLong:
		return "VALUE_LONG"
	case ValueFloat:
		return "VALUE_FLOAT"
	case ValueDouble:
		return "VALUE_DOUBLE"
	case ValueMethodType:
		return "VALUE_METHOD_TYPE"
	case ValueMethodHandle:
		return "VALUE_METHOD_HANDLE"
	case ValueString:
		return "VALUE_STRING"
	case ValueType:
		return "VALUE_TYPE"
	case ValueField:
		return "VALUE_FIELD"
	case ValueMethod:
		return "VALUE_METHOD"
	case ValueEnum:
		return "VALUE_ENUM"
	case ValueArray:
		return "VALUE_ARRAY"
	case ValueAnnotation:
		return "VALUE_ANNOTATION"
	case ValueNull:
		return "VALUE_NULL"
	case ValueBoolean:
		return "VALUE_BOOLEAN"
	}
	return fmt.Sprintf("VALUE_UNKNOWN_0x%02x", t)
}

// EncodedValue is a decoded encoded_value.
type EncodedValue struct {
	Type  uint8
	Arg   uint8 // value_arg (size-1 for numerics, boolean value for VALUE_BOOLEAN)
	Range Range

	// Exactly one of the following is meaningful, selected by Type.
	Int    int64   // byte/short/char/int/long (char zero-extended)
	Float  float32 // VALUE_FLOAT
	Double float64 // VALUE_DOUBLE
	Index  uint32  // string/type/field/method/enum/proto/method_handle index
	Bool   bool
	Array  []EncodedValue
	Annot  *EncodedAnnotation
}

// EncodedAnnotation is an encoded_annotation.
type EncodedAnnotation struct {
	Range    Range
	TypeIdx  uint32
	Elements []AnnotationElement
}

// AnnotationElement is one name/value pair.
type AnnotationElement struct {
	NameIdx uint32
	Value   EncodedValue
}

// AnnotationItem is an annotation_item.
type AnnotationItem struct {
	Range      Range
	Visibility uint8
	Annotation EncodedAnnotation
}

// Annotation visibility.
const (
	VisibilityBuild   = 0x00
	VisibilityRuntime = 0x01
	VisibilitySystem  = 0x02
)

// VisibilityName renders an annotation visibility.
func VisibilityName(v uint8) string {
	switch v {
	case VisibilityBuild:
		return "build"
	case VisibilityRuntime:
		return "runtime"
	case VisibilitySystem:
		return "system"
	}
	return fmt.Sprintf("unknown_%d", v)
}

// AnnotationSet is an annotation_set_item: offsets of annotation_items.
type AnnotationSet struct {
	Range   Range
	Offsets []uint32
}

// AnnotationSetRefList is an annotation_set_ref_list: offsets of annotation sets (per parameter).
type AnnotationSetRefList struct {
	Range   Range
	Offsets []uint32 // 0 = no annotations for that parameter
}

// AnnotationsDirectory is an annotations_directory_item.
type AnnotationsDirectory struct {
	Range                Range
	ClassAnnotationsOff  uint32
	FieldAnnotations     []FieldAnnotation
	MethodAnnotations    []MethodAnnotation
	ParameterAnnotations []ParameterAnnotation
}

// FieldAnnotation maps a field to an annotation set.
type FieldAnnotation struct {
	FieldIdx       uint32
	AnnotationsOff uint32
}

// MethodAnnotation maps a method to an annotation set.
type MethodAnnotation struct {
	MethodIdx      uint32
	AnnotationsOff uint32
}

// ParameterAnnotation maps a method to an annotation_set_ref_list.
type ParameterAnnotation struct {
	MethodIdx      uint32
	AnnotationsOff uint32
}

// EncodedArray is an encoded_array_item (also used for call sites and static values).
type EncodedArray struct {
	Range  Range
	Values []EncodedValue
}

const maxEncodedDepth = 48

// EncodedArrayItem parses the encoded_array_item at off.
func (f *File) EncodedArrayItem(off uint32) (*EncodedArray, error) {
	return f.parseEncodedArrayItem(off, nil)
}

// EncodedArrayFields re-parses an encoded_array_item recording field views.
func (f *File) EncodedArrayFields(off uint32) ([]FieldView, *EncodedArray, error) {
	var views []FieldView
	ea, err := f.parseEncodedArrayItem(off, &views)
	return views, ea, err
}

func (f *File) parseEncodedArrayItem(off uint32, rec *[]FieldView) (*EncodedArray, error) {
	c := newCursor(f.data, off, "encoded_array_item")
	c.rec = rec
	vals, err := f.readEncodedArray(c, "", 0)
	if err != nil {
		return nil, err
	}
	return &EncodedArray{Range: Range{off, c.Pos() - off}, Values: vals}, nil
}

func (f *File) readEncodedArray(c *cursor, prefix string, depth int) ([]EncodedValue, error) {
	start := c.Pos()
	n := c.uleb(prefix + "size")
	if c.Err() != nil {
		return nil, c.Err()
	}
	if uint64(n) > uint64(c.remaining()) {
		return nil, perr("encoded_array", start, fmt.Errorf("%w: size %d", ErrTooLarge, n))
	}
	vals := make([]EncodedValue, 0, n)
	for i := uint32(0); i < n; i++ {
		var name string
		if c.rec != nil {
			name = fmt.Sprintf("%svalues[%d]", prefix, i)
		}
		v, err := f.readEncodedValue(c, name, depth+1)
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, nil
}

func (f *File) readEncodedValue(c *cursor, name string, depth int) (EncodedValue, error) {
	if depth > maxEncodedDepth {
		return EncodedValue{}, perr("encoded_value", c.Pos(), ErrTooDeep)
	}
	start := c.Pos()
	hdr := c.u8(name + ".header")
	if c.Err() != nil {
		return EncodedValue{}, c.Err()
	}
	v := EncodedValue{Type: hdr & 0x1f, Arg: hdr >> 5}
	size := int(v.Arg) + 1
	readLE := func() uint64 {
		b := c.bytes(name+".value", size)
		var u uint64
		for i := len(b) - 1; i >= 0; i-- {
			u = u<<8 | uint64(b[i])
		}
		return u
	}
	signExtend := func(u uint64) int64 {
		shift := uint(64 - size*8)
		return int64(u<<shift) >> shift
	}
	checkArg := func(max int) error {
		if size > max {
			return perr("encoded_value", start, fmt.Errorf("%w: value_arg %d too large for %s", ErrInvalid, v.Arg, ValueTypeName(v.Type)))
		}
		return nil
	}
	switch v.Type {
	case ValueByte:
		if err := checkArg(1); err != nil {
			return v, err
		}
		v.Int = signExtend(readLE())
	case ValueShort:
		if err := checkArg(2); err != nil {
			return v, err
		}
		v.Int = signExtend(readLE())
	case ValueChar:
		if err := checkArg(2); err != nil {
			return v, err
		}
		v.Int = int64(readLE())
	case ValueInt:
		if err := checkArg(4); err != nil {
			return v, err
		}
		v.Int = signExtend(readLE())
	case ValueLong:
		if err := checkArg(8); err != nil {
			return v, err
		}
		v.Int = signExtend(readLE())
	case ValueFloat:
		if err := checkArg(4); err != nil {
			return v, err
		}
		// Right-zero-extended: the bytes present are the most significant.
		u := readLE() << uint(32-size*8)
		v.Float = math.Float32frombits(uint32(u))
	case ValueDouble:
		if err := checkArg(8); err != nil {
			return v, err
		}
		u := readLE() << uint(64-size*8)
		v.Double = math.Float64frombits(u)
	case ValueMethodType, ValueMethodHandle, ValueString, ValueType, ValueField, ValueMethod, ValueEnum:
		if err := checkArg(4); err != nil {
			return v, err
		}
		v.Index = uint32(readLE())
		if c.Err() == nil {
			if err := f.checkValueIndex(v.Type, v.Index); err != nil {
				return v, perr("encoded_value", start, err)
			}
		}
	case ValueArray:
		if v.Arg != 0 {
			return v, perr("encoded_value", start, fmt.Errorf("%w: VALUE_ARRAY with value_arg %d", ErrInvalid, v.Arg))
		}
		arr, err := f.readEncodedArray(c, name+".", depth+1)
		if err != nil {
			return v, err
		}
		v.Array = arr
	case ValueAnnotation:
		if v.Arg != 0 {
			return v, perr("encoded_value", start, fmt.Errorf("%w: VALUE_ANNOTATION with value_arg %d", ErrInvalid, v.Arg))
		}
		ann, err := f.readEncodedAnnotation(c, name+".", depth+1)
		if err != nil {
			return v, err
		}
		v.Annot = ann
	case ValueNull:
		if v.Arg != 0 {
			return v, perr("encoded_value", start, fmt.Errorf("%w: VALUE_NULL with value_arg %d", ErrInvalid, v.Arg))
		}
	case ValueBoolean:
		if v.Arg > 1 {
			return v, perr("encoded_value", start, fmt.Errorf("%w: VALUE_BOOLEAN with value_arg %d", ErrInvalid, v.Arg))
		}
		v.Bool = v.Arg == 1
	default:
		return v, perr("encoded_value", start, fmt.Errorf("%w: unknown value type 0x%02x", ErrInvalid, v.Type))
	}
	if c.Err() != nil {
		return v, c.Err()
	}
	v.Range = Range{start, c.Pos() - start}
	return v, nil
}

func (f *File) checkValueIndex(t uint8, idx uint32) error {
	var n uint32
	switch t {
	case ValueMethodType:
		n = f.Header.ProtoIDsSize
	case ValueMethodHandle:
		n = uint32(len(f.MethodHandles))
	case ValueString:
		n = f.Header.StringIDsSize
	case ValueType:
		n = f.Header.TypeIDsSize
	case ValueField, ValueEnum:
		n = f.Header.FieldIDsSize
	case ValueMethod:
		n = f.Header.MethodIDsSize
	}
	if idx >= n {
		return fmt.Errorf("%w: %s index %d out of range", ErrInvalid, ValueTypeName(t), idx)
	}
	return nil
}

func (f *File) readEncodedAnnotation(c *cursor, prefix string, depth int) (*EncodedAnnotation, error) {
	if depth > maxEncodedDepth {
		return nil, perr("encoded_annotation", c.Pos(), ErrTooDeep)
	}
	start := c.Pos()
	ann := &EncodedAnnotation{}
	ann.TypeIdx = c.uleb(prefix + "type_idx")
	n := c.uleb(prefix + "size")
	if c.Err() != nil {
		return nil, c.Err()
	}
	if ann.TypeIdx >= f.Header.TypeIDsSize {
		return nil, perr("encoded_annotation", start, fmt.Errorf("%w: type_idx %d", ErrInvalid, ann.TypeIdx))
	}
	if uint64(n)*2 > uint64(c.remaining()) {
		return nil, perr("encoded_annotation", start, fmt.Errorf("%w: %d elements", ErrTooLarge, n))
	}
	ann.Elements = make([]AnnotationElement, 0, n)
	for i := uint32(0); i < n; i++ {
		var nName, nValue string
		if c.rec != nil {
			nName = fmt.Sprintf("%selements[%d].name_idx", prefix, i)
			nValue = fmt.Sprintf("%selements[%d]", prefix, i)
		}
		nameIdx := c.uleb(nName)
		if c.Err() != nil {
			return nil, c.Err()
		}
		if nameIdx >= f.Header.StringIDsSize {
			return nil, perr("encoded_annotation", c.Pos(), fmt.Errorf("%w: name_idx %d", ErrInvalid, nameIdx))
		}
		v, err := f.readEncodedValue(c, nValue, depth+1)
		if err != nil {
			return nil, err
		}
		ann.Elements = append(ann.Elements, AnnotationElement{NameIdx: nameIdx, Value: v})
	}
	ann.Range = Range{start, c.Pos() - start}
	return ann, nil
}

// AnnotationItem parses the annotation_item at off.
func (f *File) AnnotationItem(off uint32) (*AnnotationItem, error) {
	return f.parseAnnotationItem(off, nil)
}

// AnnotationItemFields re-parses an annotation_item recording field views.
func (f *File) AnnotationItemFields(off uint32) ([]FieldView, *AnnotationItem, error) {
	var views []FieldView
	ai, err := f.parseAnnotationItem(off, &views)
	return views, ai, err
}

func (f *File) parseAnnotationItem(off uint32, rec *[]FieldView) (*AnnotationItem, error) {
	c := newCursor(f.data, off, "annotation_item")
	c.rec = rec
	ai := &AnnotationItem{}
	ai.Visibility = c.u8("visibility")
	if c.Err() != nil {
		return nil, c.Err()
	}
	ann, err := f.readEncodedAnnotation(c, "annotation.", 0)
	if err != nil {
		return nil, err
	}
	ai.Annotation = *ann
	ai.Range = Range{off, c.Pos() - off}
	return ai, nil
}

// AnnotationSet parses the annotation_set_item at off.
func (f *File) AnnotationSet(off uint32) (*AnnotationSet, error) {
	return f.parseAnnotationSet(off, nil)
}

// AnnotationSetFields re-parses an annotation_set_item recording field views.
func (f *File) AnnotationSetFields(off uint32) ([]FieldView, *AnnotationSet, error) {
	var views []FieldView
	as, err := f.parseAnnotationSet(off, &views)
	return views, as, err
}

func (f *File) parseAnnotationSet(off uint32, rec *[]FieldView) (*AnnotationSet, error) {
	if off%4 != 0 {
		return nil, perr("annotation_set_item", off, ErrBadAlign)
	}
	c := newCursor(f.data, off, "annotation_set_item")
	c.rec = rec
	n := c.u32("size")
	if c.Err() != nil {
		return nil, c.Err()
	}
	if uint64(n)*4 > uint64(c.remaining()) {
		return nil, perr("annotation_set_item", off, ErrTooLarge)
	}
	as := &AnnotationSet{Offsets: make([]uint32, n)}
	for i := range as.Offsets {
		var name string
		if rec != nil {
			name = fmt.Sprintf("entries[%d].annotation_off", i)
		}
		as.Offsets[i] = c.u32(name)
		if uint64(as.Offsets[i]) >= uint64(len(f.data)) {
			return nil, perr("annotation_set_item", c.Pos()-4, ErrOutOfBounds)
		}
	}
	as.Range = Range{off, 4 + n*4}
	return as, c.Err()
}

// AnnotationSetRefList parses the annotation_set_ref_list at off.
func (f *File) AnnotationSetRefList(off uint32) (*AnnotationSetRefList, error) {
	return f.parseAnnotationSetRefList(off, nil)
}

// AnnotationSetRefListFields re-parses recording field views.
func (f *File) AnnotationSetRefListFields(off uint32) ([]FieldView, *AnnotationSetRefList, error) {
	var views []FieldView
	l, err := f.parseAnnotationSetRefList(off, &views)
	return views, l, err
}

func (f *File) parseAnnotationSetRefList(off uint32, rec *[]FieldView) (*AnnotationSetRefList, error) {
	if off%4 != 0 {
		return nil, perr("annotation_set_ref_list", off, ErrBadAlign)
	}
	c := newCursor(f.data, off, "annotation_set_ref_list")
	c.rec = rec
	n := c.u32("size")
	if c.Err() != nil {
		return nil, c.Err()
	}
	if uint64(n)*4 > uint64(c.remaining()) {
		return nil, perr("annotation_set_ref_list", off, ErrTooLarge)
	}
	l := &AnnotationSetRefList{Offsets: make([]uint32, n)}
	for i := range l.Offsets {
		var name string
		if rec != nil {
			name = fmt.Sprintf("list[%d].annotations_off", i)
		}
		l.Offsets[i] = c.u32(name)
		if uint64(l.Offsets[i]) >= uint64(len(f.data)) {
			return nil, perr("annotation_set_ref_list", c.Pos()-4, ErrOutOfBounds)
		}
	}
	l.Range = Range{off, 4 + n*4}
	return l, c.Err()
}

// AnnotationsDirectory parses the annotations_directory_item at off.
func (f *File) AnnotationsDirectory(off uint32) (*AnnotationsDirectory, error) {
	return f.parseAnnotationsDirectory(off, nil)
}

// AnnotationsDirectoryFields re-parses recording field views.
func (f *File) AnnotationsDirectoryFields(off uint32) ([]FieldView, *AnnotationsDirectory, error) {
	var views []FieldView
	d, err := f.parseAnnotationsDirectory(off, &views)
	return views, d, err
}

func (f *File) parseAnnotationsDirectory(off uint32, rec *[]FieldView) (*AnnotationsDirectory, error) {
	if off%4 != 0 {
		return nil, perr("annotations_directory_item", off, ErrBadAlign)
	}
	c := newCursor(f.data, off, "annotations_directory_item")
	c.rec = rec
	d := &AnnotationsDirectory{}
	d.ClassAnnotationsOff = c.u32("class_annotations_off")
	nf := c.u32("fields_size")
	nm := c.u32("annotated_methods_size")
	np := c.u32("annotated_parameters_size")
	if c.Err() != nil {
		return nil, c.Err()
	}
	if (uint64(nf)+uint64(nm)+uint64(np))*8 > uint64(c.remaining()) {
		return nil, perr("annotations_directory_item", off, ErrTooLarge)
	}
	d.FieldAnnotations = make([]FieldAnnotation, nf)
	for i := range d.FieldAnnotations {
		var nIdx, nOff string
		if rec != nil {
			nIdx = fmt.Sprintf("field_annotations[%d].field_idx", i)
			nOff = fmt.Sprintf("field_annotations[%d].annotations_off", i)
		}
		d.FieldAnnotations[i].FieldIdx = c.u32(nIdx)
		d.FieldAnnotations[i].AnnotationsOff = c.u32(nOff)
		if d.FieldAnnotations[i].FieldIdx >= f.Header.FieldIDsSize {
			return nil, perr("annotations_directory_item", c.Pos()-8, fmt.Errorf("%w: field_idx", ErrInvalid))
		}
	}
	d.MethodAnnotations = make([]MethodAnnotation, nm)
	for i := range d.MethodAnnotations {
		var nIdx, nOff string
		if rec != nil {
			nIdx = fmt.Sprintf("method_annotations[%d].method_idx", i)
			nOff = fmt.Sprintf("method_annotations[%d].annotations_off", i)
		}
		d.MethodAnnotations[i].MethodIdx = c.u32(nIdx)
		d.MethodAnnotations[i].AnnotationsOff = c.u32(nOff)
		if d.MethodAnnotations[i].MethodIdx >= f.Header.MethodIDsSize {
			return nil, perr("annotations_directory_item", c.Pos()-8, fmt.Errorf("%w: method_idx", ErrInvalid))
		}
	}
	d.ParameterAnnotations = make([]ParameterAnnotation, np)
	for i := range d.ParameterAnnotations {
		var nIdx, nOff string
		if rec != nil {
			nIdx = fmt.Sprintf("parameter_annotations[%d].method_idx", i)
			nOff = fmt.Sprintf("parameter_annotations[%d].annotations_off", i)
		}
		d.ParameterAnnotations[i].MethodIdx = c.u32(nIdx)
		d.ParameterAnnotations[i].AnnotationsOff = c.u32(nOff)
		if d.ParameterAnnotations[i].MethodIdx >= f.Header.MethodIDsSize {
			return nil, perr("annotations_directory_item", c.Pos()-8, fmt.Errorf("%w: method_idx", ErrInvalid))
		}
	}
	d.Range = Range{off, c.Pos() - off}
	return d, c.Err()
}

// CallSite returns the encoded array backing call_site_id idx.
func (f *File) CallSite(idx uint32) (*EncodedArray, error) {
	if int(idx) >= len(f.CallSiteIDs) {
		return nil, fmt.Errorf("call_site index %d out of range", idx)
	}
	return f.EncodedArrayItem(f.CallSiteIDs[idx])
}

// StaticValues returns the static initial values for a class def (nil if none).
func (f *File) StaticValues(classDefIdx uint32) (*EncodedArray, error) {
	if int(classDefIdx) >= len(f.ClassDefs) {
		return nil, fmt.Errorf("class_def index %d out of range", classDefIdx)
	}
	off := f.ClassDefs[classDefIdx].StaticValuesOff
	if off == 0 {
		return nil, nil
	}
	return f.EncodedArrayItem(off)
}
