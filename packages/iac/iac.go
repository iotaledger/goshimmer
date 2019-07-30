package iac

import (
	olc "github.com/google/open-location-code/go"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/iota.go/trinary"
)

func Decode(trytes trinary.Trytes) (result *Area, err errors.IdentifiableError) {
	if olcCode, conversionErr := OLCCodeFromTrytes(trytes); conversionErr != nil {
		err = conversionErr
	} else {
		if codeArea, olcErr := olc.Decode(olcCode); olcErr == nil {
			result = &Area{
				IACCode:  trytes,
				OLCCode:  olcCode,
				CodeArea: codeArea,
			}
		} else {
			err = ErrDecodeFailed.Derive(olcErr, "failed to decode the IAC")
		}
	}

	return
}
