package bundleprocessor

import "github.com/iotaledger/goshimmer/packages/errors"

var (
	ErrProcessBundleFailed = errors.Wrap(errors.New("bundle processing error"), "failed to process bundle")
)
