package event

import (
	"reflect"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/constraints"
)

// Group is a trait that can be embedded into a struct to make the contained events linkable.
type Group[A any, B ptrGroupType[A, B]] struct {
	linkUpdated *Event1[B]
	sync.Once
}

// NewGroup returns the linkable constructor for the given type.
func NewGroup[A any, B ptrGroupType[A, B]](newFunc func() B) (constructor func(...B) B) {
	return func(optLinkTargets ...B) (self B) {
		self = newFunc()

		selfValue := reflect.ValueOf(self).Elem()
		self.onLinkUpdated(func(linkTarget B) {
			if linkTarget == nil {
				linkTarget = new(A)
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
func (l *Group[A, B]) LinkTo(target B) {
	l.linkUpdatedEvent().Trigger(target)
}

// onLinkUpdated registers a callback to be called when the link to the referenced Group is set or updated.
func (l *Group[A, B]) onLinkUpdated(callback func(linkTarget B)) {
	l.linkUpdatedEvent().Hook(callback)
}

// linkUpdatedEvent returns the linkUpdated Event.
func (l *Group[A, B]) linkUpdatedEvent() (linkUpdatedEvent *Event1[B]) {
	l.Do(func() {
		l.linkUpdated = New1[B]()
	})

	return l.linkUpdated
}

// ptrGroupType is a helper type to create a pointer to a linkableCollectionType.
type ptrGroupType[A any, B constraints.Ptr[A]] interface {
	*A

	onLinkUpdated(callback func(B))
	LinkTo(target B)
}
