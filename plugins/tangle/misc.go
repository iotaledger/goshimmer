package tangle

import "github.com/iotaledger/iota.go/trinary"

func databaseKeyForHashPrefixedHash(address trinary.Hash, transactionHash trinary.Hash) []byte {
	return append(databaseKeyForHashPrefix(address), trinary.MustTrytesToBytes(transactionHash)...)
}

func databaseKeyForHashPrefix(hash trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(hash)
}
