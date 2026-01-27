package main

import (
	"fmt"
	"strconv"
)

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
		// if it's a hex represenatation, return it's value
		if len(str) == 18 && str[:2] == "0x" {
			if s, err := strconv.ParseUint(string(str[2:]), 16, 64); err == nil {
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
