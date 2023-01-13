package devnetvm

import (
	"context"

	"github.com/iotaledger/hive.go/core/cerrors"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// OutputFromBytes is the factory function for Outputs.
func OutputFromBytes(data []byte) (output utxo.Output, err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), data, &output, serix.WithValidation())
	if err != nil {
		return nil, errors.WithMessagef(cerrors.ErrParseBytesFailed, "failed to parse Output: %s", err.Error())
	}

	return output, nil
}
