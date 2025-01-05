package tobin

import (
	"bytes"
	"errors"
)

func DecodeEscapeSequence(src []byte) ([]byte, error) {
	var b bytes.Buffer
	for i := 0; i < len(src); {
		if src[i] == '\\' {
			if !(i+1 < len(src)) {
				return nil, errors.New("EOL")
			}
			i++
			switch src[i] {
			case '\\':
				b.WriteByte('\\')
				i++
			case '\'':
				b.WriteByte('\'')
				i++
			case '"':
				b.WriteByte('"')
				i++
			case 'a':
				b.WriteByte('\a')
				i++
			case 'b':
				b.WriteByte('\b')
				i++
			case 'f':
				b.WriteByte('\f')
				i++
			case 'n':
				b.WriteByte('\n')
				i++
			case 'r':
				b.WriteByte('\r')
				i++
			case 't':
				b.WriteByte('\t')
				i++
			case 'v':
				b.WriteByte('\v')
				i++
			case 'x': // \xhh
				i++
				var hex uint8
				for j := 0; j < 2 && i < len(src); j++ {
					hex *= 16
					o, ok := hexToUint8(src[i])
					if !ok {
						break
					}
					hex += o
					i++
				}
				b.WriteByte(hex)
			case '0', '1', '2', '3', '4', '5', '6', '7': // \ooo
				var oct uint8
				for j := 0; j < 3 && i < len(src); j++ {
					oct *= 8
					o, ok := octToUint8(src[i])
					if !ok {
						break
					}
					oct += o
					i++
				}
				b.WriteByte(oct)
			default:
				return nil, errors.New("invalid escape sequence")
			}
		} else {
			b.WriteByte(src[i])
			i++
		}
	}
	return b.Bytes(), nil
}

func hexToUint8(b byte) (uint8, bool) {
	if '0' <= b && b <= '9' {
		return b - uint8('0'), true
	} else if 'a' <= b && b <= 'f' {
		return 10 + b - uint8('a'), true
	} else if 'A' <= b && b <= 'F' {
		return 10 + b - uint8('A'), true
	}
	return 0, false
}

func octToUint8(b byte) (uint8, bool) {
	if '0' <= b && b <= '7' {
		return b - uint8('0'), true
	}
	return 0, false
}
