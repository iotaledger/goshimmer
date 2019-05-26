# FPC

This package implements the Fast Probabilistic Consensus protocol.

## Dependencies


* `GetKnownPeers func() -> []int` TODO: change int to node
* `QueryNode func([]Hash, int) -> []Opinion` TODO: change int to node

## Interfaces

* `New(GetKnownPeers, QueryNode, *Parameters) -> *FPC` : returns a new FPC instance
* `(*FPC) VoteOnTxs([]TxOpinion)` : adds given txs to the FPC waiting txs list and set the initial opinion to the opinion history
* `(*FPC) Tick(uint64, float64)` : updates FPC state with the new random and starts a new round
* `(*FPC) GetInterimOpinion([]Hash) -> []Opinion` : returns the current opinion of the given txs
* `(*FPC) Debug_GetOpinionHistory() -> *OpinionMap` : returns the entire opinion history

* Finalized txs are notified via the channel `FinalizedTxs` of the particular FPC instance. TODO: maybe we can find a better way to do that.

## Test

From the `goshimmer` folder run:

```
go test -count=1 -v -cover ./packages/fpc
```

TODO: unit test

