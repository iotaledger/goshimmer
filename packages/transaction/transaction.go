package transaction

import (
    "github.com/iotaledger/goshimmer/packages/curl"
    "github.com/iotaledger/goshimmer/packages/ternary"
)

type Transaction struct {
    SignatureMessageFragment      ternary.Trits
    Address                       ternary.Trits
    Value                         ternary.Trits
    Timestamp                     ternary.Trits
    CurrentIndex                  ternary.Trits
    LatestIndex                   ternary.Trits
    BundleHash                    ternary.Trits
    TrunkTransactionHash          ternary.Trits
    BranchTransactionHash         ternary.Trits
    Tag                           ternary.Trits
    Nonce                         ternary.Trits

    Hash                          ternary.Trits
    WeightMagnitude               int
    Bytes                         []byte
    Trits                         ternary.Trits
}

func FromTrits(trits ternary.Trits, optionalHash ...ternary.Trits) *Transaction {
    hash := <- curl.CURLP81.Hash(trits)

    transaction := &Transaction{
        SignatureMessageFragment:      trits[SIGNATURE_MESSAGE_FRAGMENT_OFFSET:SIGNATURE_MESSAGE_FRAGMENT_END],
        Address:                       trits[ADDRESS_OFFSET:ADDRESS_END],
        Value:                         trits[VALUE_OFFSET:VALUE_END],
        Timestamp:                     trits[TIMESTAMP_OFFSET:TIMESTAMP_END],
        CurrentIndex:                  trits[CURRENT_INDEX_OFFSET:CURRENT_INDEX_END],
        LatestIndex:                   trits[LATEST_INDEX_OFFSET:LATEST_INDEX_END],
        BundleHash:                    trits[BUNDLE_HASH_OFFSET:BUNDLE_HASH_END],
        TrunkTransactionHash:          trits[TRUNK_TRANSACTION_HASH_OFFSET:TRUNK_TRANSACTION_HASH_END],
        BranchTransactionHash:         trits[BRANCH_TRANSACTION_HASH_OFFSET:BRANCH_TRANSACTION_HASH_END],
        Tag:                           trits[TAG_OFFSET:TAG_END],
        Nonce:                         trits[NONCE_OFFSET:NONCE_END],

        Hash:                          hash,
        WeightMagnitude:               hash.TrailingZeroes(),
        Trits:                         trits,
    }

    return transaction
}

func FromBytes(bytes []byte) *Transaction {
    transaction := FromTrits(ternary.BytesToTrits(bytes)[:MARSHALLED_TOTAL_SIZE])
    transaction.Bytes = bytes

    return transaction
}