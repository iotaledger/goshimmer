package causalorder

type Entity[IDType comparable] interface {
	ID() IDType
	ParentIDs() []IDType

	RLock()
	RUnlock()
	Lock()
	Unlock()
}

type EntityProvider[IDType comparable, EntityType Entity[IDType]] func(IDType) (entity EntityType, exists bool)
