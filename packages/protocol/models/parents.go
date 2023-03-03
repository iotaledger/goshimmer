package models

import "fmt"

// ParentsType is a type that defines the type of the parent.
type ParentsType uint8

const (
	// UndefinedParentType is the undefined parent.
	UndefinedParentType ParentsType = iota
	// StrongParentType is the ParentsType for a strong parent.
	StrongParentType
	// WeakParentType is the ParentsType for a weak parent.
	WeakParentType
	// ShallowLikeParentType is the ParentsType for the shallow like parent.
	ShallowLikeParentType
)

// String returns string representation of ParentsType.
func (bp ParentsType) String() string {
	return fmt.Sprintf("ParentType(%s)", []string{"Undefined", "Strong", "Weak", "Shallow Like"}[bp])
}

// Parent is a parent that can be either strong or weak.
type Parent struct {
	ID   BlockID
	Type ParentsType
}

// ParentBlockIDs is a map of ParentType to BlockIDs.
type ParentBlockIDs map[ParentsType]BlockIDs

// NewParentBlockIDs constructs a new ParentBlockIDs.
func NewParentBlockIDs() ParentBlockIDs {
	p := make(ParentBlockIDs)
	return p
}

// AddStrong adds a strong parent to the map.
func (p ParentBlockIDs) AddStrong(blockID BlockID) ParentBlockIDs {
	if _, exists := p[StrongParentType]; !exists {
		p[StrongParentType] = NewBlockIDs()
	}
	return p.Add(StrongParentType, blockID)
}

// Add adds a parent to the map.
func (p ParentBlockIDs) Add(parentType ParentsType, blockID BlockID) ParentBlockIDs {
	if _, exists := p[parentType]; !exists {
		p[parentType] = NewBlockIDs()
	}
	p[parentType].Add(blockID)
	return p
}

// AddAll adds a collection of parents to the map.
func (p ParentBlockIDs) AddAll(parentType ParentsType, blockIDs BlockIDs) ParentBlockIDs {
	if _, exists := p[parentType]; !exists {
		p[parentType] = NewBlockIDs()
	}
	p[parentType].AddAll(blockIDs)
	return p
}

// IsEmpty returns true if the ParentBlockIDs are empty.
func (p ParentBlockIDs) IsEmpty() bool {
	return len(p) == 0
}

// Clone returns a copy of map.
func (p ParentBlockIDs) Clone() ParentBlockIDs {
	pCloned := NewParentBlockIDs()
	for parentType, blockIDs := range p {
		if _, exists := p[parentType]; !exists {
			p[parentType] = NewBlockIDs()
		}
		pCloned.AddAll(parentType, blockIDs)
	}
	return pCloned
}

// ForEach executes a consumer func for each parent.
func (p ParentBlockIDs) ForEach(callback func(parent Parent)) {
	for parentType, parents := range p {
		for parentID := range parents {
			callback(Parent{Type: parentType, ID: parentID})
		}
	}
}

func (p ParentBlockIDs) CleanupReferences() {
	for strongParent := range p[StrongParentType] {
		delete(p[WeakParentType], strongParent)
	}
	for shallowLike := range p[ShallowLikeParentType] {
		delete(p[WeakParentType], shallowLike)
	}

	if len(p[WeakParentType]) == 0 {
		delete(p, WeakParentType)
	}

	if len(p[ShallowLikeParentType]) == 0 {
		delete(p, ShallowLikeParentType)
	}
}
