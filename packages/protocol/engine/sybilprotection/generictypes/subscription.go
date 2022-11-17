package generictypes

import (
	"sync"
)

type Subscription struct {
	onCancel   func()
	cancelOnce sync.Once
}

func NewSubscription(onCancel func()) *Subscription {
	return &Subscription{
		onCancel: onCancel,
	}
}

func (s *Subscription) Cancel() {
	s.cancelOnce.Do(s.onCancel)
}
