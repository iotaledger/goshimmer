package storage

type Realm byte

const (
	UnspentOutputsRealm Realm = iota
	UnspentOutputIDsRealm
	ConsensusWeightsRealm
	BlockRealm
	LedgerStateDiffsRealm
	SolidEntryPointsRealm
	ActivityLogRealm
)
