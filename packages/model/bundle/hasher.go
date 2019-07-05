package bundle

import (
	"github.com/iotaledger/goshimmer/packages/curl"
)

const (
	CURLP81_HASH_LENGTH = 243
	CURLP81_ROUNDS      = 81
)

var (
	Hasher = curl.NewBatchHasher(CURLP81_HASH_LENGTH, CURLP81_ROUNDS)
)
