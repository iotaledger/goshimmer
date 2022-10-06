package chainstorage

type Type byte

const (
	LedgerStateStorage Type = iota
	StateTreeStorage
	ManaTreeStorage
	CommitmentRootsStorage
	MutationTreesStorage
	BlockStorageType
	LedgerDiffStorage
	SolidEntryPointsStorage
	ActivityLogStorage
)
