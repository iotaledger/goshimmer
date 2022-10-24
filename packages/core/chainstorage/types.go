package chainstorage

type Type byte

const (
	LedgerStateStorageType Type = iota
	StateTreeStorageType
	ManaTreeStorageType
	CommitmentRootsStorageType
	MutationTreesStorageType
	BlockStorageType
	LedgerDiffStorageType
	SolidEntryPointsStorageType
	ActivityLogStorageType
)
