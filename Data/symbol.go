package data

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// hexFilenameRE matches a non-empty lowercase hex string (no path separators, no dots).
var hexFilenameRE = regexp.MustCompile(`^[0-9a-f]+$`)

// SafeHexFilename validates and normalises a string intended to be used as a
// filename that should only contain hexadecimal digits.  It returns the
// lower-cased hex string, or an error if the value is empty or contains any
// non-hex characters (which would allow path traversal or injection).
func SafeHexFilename(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("hex filename must not be empty")
	}
	lower := strings.ToLower(s)
	if !hexFilenameRE.MatchString(lower) {
		return "", fmt.Errorf("invalid hex filename %q: must contain only hex digits [0-9a-f]", s)
	}
	return lower, nil
}

// Symbol is the EchoVR FNV-1a-derived 64-bit hash type used to uniquely identify assets and cosmetics.
type Symbol uint64

var symbolSeed [0x100]uint64 = generateSymbolSeed()

func generateSymbolSeed() [0x100]uint64 {
	seed := [0x100]uint64{}
	i := uint64(0)
	s := uint64(0x95ac9329ac4bc9b5)
	for i < 0x100 {
		num1 := uint64(0)
		if (i & 0x80) != 0 {
			num1 = 0x2b5926535897936a
		}

		if (i & 0x40) != 0 {
			num1 = 0xbef5b57af4dc5adf
			if (i & 0x80) == 0 {
				num1 = s
			}
		}

		num2 := num1*2 ^ s
		if (i & 0x20) == 0 {
			num2 = num1 * 2
		}

		num1 = num2*2 ^ s
		if (i & 0x10) == 0 {
			num1 = num2 * 2
		}

		num2 = num1*2 ^ s
		if (i & 8) == 0 {
			num2 = num1 * 2
		}

		num1 = num2*2 ^ s
		if (i & 4) == 0 {
			num1 = num2 * 2
		}

		num2 = num1*2 ^ s
		if (i & 2) == 0 {
			num2 = num1 * 2
		}

		num1 = num2*2 ^ s
		if (i & 1) == 0 {
			num1 = num2 * 2
		}

		seed[i] = num1 * 2
		i += 1
	}

	return seed
}

// ToSymbol converts a value to a Symbol. Strings are hashed using the game's symbol algorithm;
// numeric types are cast directly. Passing an unsupported type panics.
func ToSymbol(v any) Symbol {
	// if it's a number, return it as an uint64
	switch t := v.(type) {
	case Symbol:
		return t
	case int:
		return Symbol(t)
	case int64:
		return Symbol(t)
	case uint64:
		return Symbol(t)
	case string:
		str := t
		// Empty string returns 0
		if len(str) == 0 {
			return Symbol(0)
		}
		// if it's a hex representation, return its value
		if (len(str) >= 3 && str[:2] == "0x") || (len(str) >= 1 && !strings.Contains(str, " ")) {
			// Try parsing as hex if it looks like one
			hexPart := str
			if strings.HasPrefix(str, "0x") {
				hexPart = str[2:]
			}
			if s, err := strconv.ParseUint(hexPart, 16, 64); err == nil {
				return Symbol(s)
			}
		}
		// Convert it to a symbol
		symbol := uint64(0xffffffffffffffff)
		// lowercase the string
		for i := range str {
			a := str[i] + ' '
			if str[i] < 'A' || str[i] >= '[' {
				a = str[i]
			}
			symbol = uint64(a) ^ symbolSeed[symbol>>0x38&0xff] ^ symbol<<8
		}
		return Symbol(symbol)
	default:
		panic(fmt.Errorf("invalid type: %T", v))
	}
}

// HexToSymbol parses a lowercase hex string (without "0x" prefix) into a Symbol int64.
// Returns -1 if the string is empty or cannot be parsed.
func HexToSymbol(s string) int64 {
	if s == "" {
		return -1
	}
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return -1
	}
	return int64(v)
}

// SymbolToHex formats a Symbol int64 as a lowercase hex string without "0x" prefix.
// Returns an empty string for -1 (null symbol).
func SymbolToHex(v int64) string {
	if v == -1 {
		return ""
	}
	return fmt.Sprintf("%016x", uint64(v))
}
