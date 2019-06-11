package iac

import (
	olc "github.com/google/open-location-code/go"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

func Decode(trinary ternary.Trinary) (result *Area, err errors.IdentifiableError) {
	if olcCode, conversionErr := OLCCodeFromTrinary(trinary); err != nil {
		err = conversionErr
	} else {
		if codeArea, olcErr := olc.Decode(olcCode); olcErr == nil {
			result = &Area{
				IACCode:  trinary,
				OLCCode:  olcCode,
				CodeArea: codeArea,
			}
		} else {
			err = ErrDecodeFailed.Derive(olcErr, "failed to decode the IAC")
		}
	}

	return
}
