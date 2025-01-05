package tobin

import (
	"bytes"
	"testing"
)

var decodeEscapeSequenceTests = []struct {
	src []byte
	dst []byte
}{
	{[]byte("ABC"), []byte("ABC")},
	{[]byte(`\\\'\"\a\b\f\n\r\t\v`), []byte("\x5c\x27\x22\x07\x08\x0c\x0a\x0d\x09\x0b")},
	{[]byte(`\123\000\777`), []byte("S\x00\xff")},
	{[]byte(`\0\00\0000`), []byte("\x00\x00\x000")},
	{[]byte(`\xab\xCD\x00\xff`), []byte("\xab\xcd\x00\xff")},
	{[]byte(`\x0\x00\x000`), []byte("\x00\x00\x000")},
}

func TestDecodeEscapeSequence(t *testing.T) {
	for _, tt := range decodeEscapeSequenceTests {
		if dst, err := DecodeEscapeSequence(tt.src); !bytes.Equal(dst, tt.dst) || err != nil {
			t.Errorf("decodeEscapeSequence(%#q) = %v, want %v", tt.src, dst, tt.dst)
		}
	}
}
