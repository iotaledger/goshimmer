package event

import (
	"reflect"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/constraints"
)

// Group is a trait that can be embedded into a struct to make the contained events linkable.
type Group[GroupType any, GroupPtrType ptrGroupType[GroupType, GroupPtrType]] struct {
	linkUpdated *Event1[GroupPtrType]
	sync.Once
}

// NewGroup returns the linkable constructor for the given type.
func NewGroup[GroupType any, GroupPtrType ptrGroupType[GroupType, GroupPtrType]](newFunc func() GroupPtrType) (constructor func(...GroupPtrType) GroupPtrType) {
	return func(optLinkTargets ...GroupPtrType) (self GroupPtrType) {
		self = newFunc()

		selfValue := reflect.ValueOf(self).Elem()
		self.onLinkUpdated(func(linkTarget GroupPtrType) {
			if linkTarget == nil {
				linkTarget = new(GroupType)
			}

			linkTargetValue := reflect.ValueOf(linkTarget).Elem()

			for i := 0; i < selfValue.NumField(); i++ {
				if sourceField := selfValue.Field(i); sourceField.Kind() == reflect.Ptr {
					if linkTo := sourceField.MethodByName("LinkTo"); linkTo.IsValid() {
						linkTo.Call([]reflect.Value{linkTargetValue.Field(i)})
					}
				}
			}
		})

		if len(optLinkTargets) > 0 {
			self.LinkTo(optLinkTargets[0])
		}

		return self
	}
}

// LinkTo links the group to another group of the same type (nil unlinks).
func (g *Group[GroupType, GroupPtrType]) LinkTo(target GroupPtrType) {
	g.linkUpdatedEvent().Trigger(target)
}

// onLinkUpdated registers a callback to be called when the link to the referenced Group is set or updated.
func (g *Group[GroupType, GroupPtrType]) onLinkUpdated(callback func(linkTarget GroupPtrType)) {
	g.linkUpdatedEvent().Hook(callback)
}

// linkUpdatedEvent returns the linkUpdated Event.
func (g *Group[GroupType, GroupPtrType]) linkUpdatedEvent() *Event1[GroupPtrType] {
	g.Do(func() {
		g.linkUpdated = New1[GroupPtrType]()
	})

	return g.linkUpdated
}

// ptrGroupType is a helper type to create a pointer to a Group type.
type ptrGroupType[GroupType any, GroupPtrType constraints.Ptr[GroupType]] interface {
	*GroupType

	onLinkUpdated(callback func(GroupPtrType))
	LinkTo(target GroupPtrType)
}
