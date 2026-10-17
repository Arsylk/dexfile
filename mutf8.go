package dexfile

import (
	"unicode/utf16"
	"unicode/utf8"
)

// DecodeMUTF8 decodes a Modified UTF-8 byte sequence (as used by DEX
// string_data_item, without the trailing NUL) into a Go string.
//
// It returns the decoded string, the number of UTF-16 code units the data
// represents, and whether any malformed sequence was encountered. Lone
// surrogates and invalid sequences are replaced with U+FFFD; the caller can
// always fall back to the raw bytes via the string's byte range.
func DecodeMUTF8(b []byte) (s string, utf16Len int, malformed bool) {
	// Fast path: pure ASCII.
	ascii := true
	for _, c := range b {
		if c >= 0x80 || c == 0 {
			ascii = false
			break
		}
	}
	if ascii {
		return string(b), len(b), false
	}

	out := make([]byte, 0, len(b)+8)
	var pendingHigh rune = -1
	flush := func() {
		if pendingHigh >= 0 {
			out = utf8.AppendRune(out, utf8.RuneError)
			malformed = true
			pendingHigh = -1
		}
	}
	i := 0
	for i < len(b) {
		c := b[i]
		var unit rune
		switch {
		case c&0x80 == 0:
			unit = rune(c)
			i++
		case c&0xe0 == 0xc0:
			if i+1 >= len(b) || b[i+1]&0xc0 != 0x80 {
				flush()
				out = utf8.AppendRune(out, utf8.RuneError)
				malformed = true
				utf16Len++
				i++
				continue
			}
			unit = rune(c&0x1f)<<6 | rune(b[i+1]&0x3f)
			i += 2
		case c&0xf0 == 0xe0:
			if i+2 >= len(b) || b[i+1]&0xc0 != 0x80 || b[i+2]&0xc0 != 0x80 {
				flush()
				out = utf8.AppendRune(out, utf8.RuneError)
				malformed = true
				utf16Len++
				i++
				continue
			}
			unit = rune(c&0x0f)<<12 | rune(b[i+1]&0x3f)<<6 | rune(b[i+2]&0x3f)
			i += 3
		default:
			// 4-byte sequences and stray continuation bytes are not legal MUTF-8.
			flush()
			out = utf8.AppendRune(out, utf8.RuneError)
			malformed = true
			utf16Len++
			i++
			continue
		}
		utf16Len++
		if utf16.IsSurrogate(unit) {
			if unit < 0xdc00 { // high surrogate
				flush()
				pendingHigh = unit
				continue
			}
			// low surrogate
			if pendingHigh >= 0 {
				out = utf8.AppendRune(out, utf16.DecodeRune(pendingHigh, unit))
				pendingHigh = -1
			} else {
				out = utf8.AppendRune(out, utf8.RuneError)
				malformed = true
			}
			continue
		}
		flush()
		out = utf8.AppendRune(out, unit)
	}
	flush()
	return string(out), utf16Len, malformed
}
