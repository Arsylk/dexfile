// Package dexfile implements a bounds-safe, byte-range-aware parser for
// Android DEX files (versions 035 through 041, with best-effort handling of
// later versions).
//
// Every structure that the parser decodes records the exact file range it was
// decoded from, so that callers can return the original bytes of any entity
// without reconstructing them. All reads go through a cursor that validates
// bounds; malformed input yields *ParseError values, never panics.
package dexfile

import (
	"errors"
	"fmt"
)

// Range is a byte range within the DEX file: [Off, Off+Len).
type Range struct {
	Off uint32 `json:"off"`
	Len uint32 `json:"len"`
}

// End returns the exclusive end offset of the range.
func (r Range) End() uint32 { return r.Off + r.Len }

// Contains reports whether the file offset lies inside the range.
func (r Range) Contains(off uint32) bool { return off >= r.Off && off < r.Off+r.Len }

// IsZero reports whether the range is unset.
func (r Range) IsZero() bool { return r.Len == 0 && r.Off == 0 }

// FieldView describes one decoded field together with the exact bytes it was
// decoded from. It is produced only when a cursor has a recorder attached.
type FieldView struct {
	Name  string `json:"name"`
	Off   uint32 `json:"file_offset"`
	Len   uint32 `json:"len"`
	Value any    `json:"value"`
}

// ParseError describes a structural problem in the input.
type ParseError struct {
	What   string // which structure / field
	Offset uint32 // file offset where the problem was detected
	Err    error  // underlying cause
}

func (e *ParseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("dex: %s at offset 0x%x: %v", e.What, e.Offset, e.Err)
	}
	return fmt.Sprintf("dex: %s at offset 0x%x", e.What, e.Offset)
}

func (e *ParseError) Unwrap() error { return e.Err }

// Sentinel causes wrapped by ParseError.
var (
	ErrOutOfBounds = errors.New("out of bounds")
	ErrBadMagic    = errors.New("bad magic")
	ErrTooLarge    = errors.New("size exceeds file")
	ErrBadLEB128   = errors.New("malformed LEB128")
	ErrBadAlign    = errors.New("misaligned offset")
	ErrTooDeep     = errors.New("nesting too deep")
	ErrTruncated   = errors.New("truncated")
	ErrInvalid     = errors.New("invalid value")
)

func perr(what string, off uint32, err error) *ParseError {
	return &ParseError{What: what, Offset: off, Err: err}
}

// cursor is a bounds-checked little-endian reader over the file bytes.
// It is sticky-error: after the first failure every read returns zero and
// Err() reports the failure. Callers check Err() after a group of reads.
type cursor struct {
	data []byte
	pos  int
	err  error
	rec  *[]FieldView // optional field recorder
	what string
}

func newCursor(data []byte, off uint32, what string) *cursor {
	c := &cursor{data: data, what: what}
	if int64(off) > int64(len(data)) {
		c.err = perr(what, off, ErrOutOfBounds)
		c.pos = len(data)
		return c
	}
	c.pos = int(off)
	return c
}

func (c *cursor) Err() error { return c.err }

func (c *cursor) Pos() uint32 { return uint32(c.pos) }

func (c *cursor) fail(err error) {
	if c.err == nil {
		c.err = perr(c.what, uint32(c.pos), err)
	}
}

func (c *cursor) remaining() int { return len(c.data) - c.pos }

func (c *cursor) record(name string, off int, n int, v any) {
	if c.rec != nil {
		*c.rec = append(*c.rec, FieldView{Name: name, Off: uint32(off), Len: uint32(n), Value: v})
	}
}

func (c *cursor) u8(name string) uint8 {
	if c.err != nil {
		return 0
	}
	if c.remaining() < 1 {
		c.fail(ErrOutOfBounds)
		return 0
	}
	v := c.data[c.pos]
	c.record(name, c.pos, 1, v)
	c.pos++
	return v
}

func (c *cursor) u16(name string) uint16 {
	if c.err != nil {
		return 0
	}
	if c.remaining() < 2 {
		c.fail(ErrOutOfBounds)
		return 0
	}
	v := uint16(c.data[c.pos]) | uint16(c.data[c.pos+1])<<8
	c.record(name, c.pos, 2, v)
	c.pos += 2
	return v
}

func (c *cursor) u32(name string) uint32 {
	if c.err != nil {
		return 0
	}
	if c.remaining() < 4 {
		c.fail(ErrOutOfBounds)
		return 0
	}
	d := c.data[c.pos:]
	v := uint32(d[0]) | uint32(d[1])<<8 | uint32(d[2])<<16 | uint32(d[3])<<24
	c.record(name, c.pos, 4, v)
	c.pos += 4
	return v
}

func (c *cursor) bytes(name string, n int) []byte {
	if c.err != nil {
		return nil
	}
	if n < 0 || c.remaining() < n {
		c.fail(ErrOutOfBounds)
		return nil
	}
	v := c.data[c.pos : c.pos+n]
	c.record(name, c.pos, n, fmt.Sprintf("%x", v))
	c.pos += n
	return v
}

// skip advances without recording.
func (c *cursor) skip(n int) {
	if c.err != nil {
		return
	}
	if n < 0 || c.remaining() < n {
		c.fail(ErrOutOfBounds)
		return
	}
	c.pos += n
}

// uleb reads an unsigned LEB128 value of at most 5 bytes (32-bit range), as
// required by the DEX format.
func (c *cursor) uleb(name string) uint32 {
	if c.err != nil {
		return 0
	}
	start := c.pos
	var result uint64
	var shift uint
	for i := 0; i < 5; i++ {
		if c.pos >= len(c.data) {
			c.fail(ErrOutOfBounds)
			return 0
		}
		b := c.data[c.pos]
		c.pos++
		result |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			if result > 0xffffffff {
				c.pos = start
				c.fail(ErrBadLEB128)
				return 0
			}
			c.record(name, start, c.pos-start, uint32(result))
			return uint32(result)
		}
		shift += 7
	}
	c.pos = start
	c.fail(ErrBadLEB128)
	return 0
}

// sleb reads a signed LEB128 value of at most 5 bytes.
func (c *cursor) sleb(name string) int32 {
	if c.err != nil {
		return 0
	}
	start := c.pos
	var result int64
	var shift uint
	for i := 0; i < 5; i++ {
		if c.pos >= len(c.data) {
			c.fail(ErrOutOfBounds)
			return 0
		}
		b := c.data[c.pos]
		c.pos++
		result |= int64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			if b&0x40 != 0 && shift < 64 {
				result |= -(int64(1) << shift)
			}
			if result < -0x80000000 || result > 0x7fffffff {
				c.pos = start
				c.fail(ErrBadLEB128)
				return 0
			}
			c.record(name, start, c.pos-start, int32(result))
			return int32(result)
		}
	}
	c.pos = start
	c.fail(ErrBadLEB128)
	return 0
}

// ulebp1 reads a uleb128p1 value: ULEB128 minus one, so that 0 encodes -1.
func (c *cursor) ulebp1(name string) int32 {
	if c.err != nil {
		return 0
	}
	start := c.pos
	saved := c.rec
	c.rec = nil
	v := c.uleb(name)
	c.rec = saved
	if c.err != nil {
		return 0
	}
	r := int32(int64(v) - 1)
	c.record(name, start, c.pos-start, r)
	return r
}

// checkRange validates that [off, off+n) lies within the data.
func checkRange(data []byte, off uint32, n uint64, what string) error {
	if uint64(off)+n > uint64(len(data)) {
		return perr(what, off, ErrOutOfBounds)
	}
	return nil
}

// Little-endian helpers used by the instruction decoder (callers pre-check bounds).
func le16(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
func le64(b []byte) uint64 { return uint64(le32(b)) | uint64(le32(b[4:]))<<32 }
