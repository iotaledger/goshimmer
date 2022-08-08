package devnetvm

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/cerrors"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
)

// OutputFromBytes is the factory function for Outputs.
func OutputFromBytes(data []byte) (output utxo.Output, err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), data, &output, serix.WithValidation())
	if err != nil {
		return nil, errors.Errorf("failed to parse Output (%v): %w", err, cerrors.ErrParseBytesFailed)
	}

	return output, nil
}
