package iac

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	OLC_ALPHABET   = []rune{'2', '3', '4', '5', '6', '7', '8', '9', 'C', 'F', 'G', 'H', 'J', 'M', 'P', 'Q', 'R', 'V', 'W', 'X'}
	IAC_ALPHABET   = []rune{'F', 'G', 'H', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'X', 'W', 'Y', 'Z'}
	OLC_TO_IAC_MAP = make(map[rune]rune, 22)
	IAC_TO_OLC_MAP = make(map[rune]rune, 22)
)

const (
	OLC_SEPARATOR = '+'
	OLC_PADDING   = '0'
	IAC_SEPARATOR = '9'
	IAC_PADDING   = 'A'
)

func init() {
	for pos, char := range OLC_ALPHABET {
		OLC_TO_IAC_MAP[char] = IAC_ALPHABET[pos]
		IAC_TO_OLC_MAP[IAC_ALPHABET[pos]] = char
	}

	OLC_TO_IAC_MAP[OLC_SEPARATOR] = IAC_SEPARATOR
	OLC_TO_IAC_MAP[OLC_PADDING] = IAC_PADDING
	IAC_TO_OLC_MAP[IAC_SEPARATOR] = OLC_SEPARATOR
	IAC_TO_OLC_MAP[IAC_PADDING] = OLC_PADDING
}

func TrytesFromOLCCode(code string) (result trinary.Trytes, err errors.IdentifiableError) {
	for _, char := range code {
		if translatedChar, exists := OLC_TO_IAC_MAP[char]; exists {
			result += trinary.Trytes(translatedChar)
		} else {
			err = ErrConversionFailed.Derive("invalid character in input")
		}
	}

	return
}

func OLCCodeFromTrytes(trytes trinary.Trytes) (result string, err errors.IdentifiableError) {
	for _, char := range trytes {
		if translatedChar, exists := IAC_TO_OLC_MAP[char]; exists {
			result += string(translatedChar)
		} else {
			err = ErrConversionFailed.Derive("invalid character in input")
		}
	}

	return
}
