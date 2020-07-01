package gossip

import (
	"container/list"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
)

const (
	// the amount of oldest tips in the tip pool to broadcast up on each interval
	maxOldestTipsToBroadcastPerInterval = 2
)

var tips = tiplist{dict: make(map[message.Id]*list.Element)}

type tiplist struct {
	mu sync.Mutex

	dict     map[message.Id]*list.Element
	list     list.List
	iterator *list.Element
}

func (s *tiplist) AddTip(id message.Id) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, contains := s.dict[id]; contains {
		return
	}

	elem := s.list.PushBack(id)
	s.dict[id] = elem
	if s.iterator == nil {
		s.iterator = elem
	}
}

func (s *tiplist) RemoveTip(id message.Id) {
	s.mu.Lock()
	defer s.mu.Unlock()

	elem, ok := s.dict[id]
	if ok {
		s.list.Remove(elem)
		if s.iterator == elem {
			s.next(elem)
		}
	}
}

func (s *tiplist) Next() (id message.Id) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.iterator != nil {
		id = s.iterator.Value.(message.Id)
		s.next(s.iterator)
	}
	return
}

func (s *tiplist) next(elem *list.Element) {
	s.iterator = elem.Next()
	if s.iterator == nil {
		s.iterator = s.list.Front()
	}
}

func configureTipBroadcaster() {
	tipSelector := messagelayer.TipSelector()
	addedTipsClosure := events.NewClosure(tips.AddTip)
	removedTipClosure := events.NewClosure(tips.RemoveTip)
	tipSelector.Events.TipAdded.Attach(addedTipsClosure)
	tipSelector.Events.TipRemoved.Attach(removedTipClosure)

	if err := daemon.BackgroundWorker("Tips-Broadcaster", func(shutdownSignal <-chan struct{}) {
		log.Info("broadcaster started")
		defer log.Info("broadcaster stopped")
		defer tipSelector.Events.TipAdded.Detach(addedTipsClosure)
		defer tipSelector.Events.TipRemoved.Detach(removedTipClosure)
		ticker := time.NewTicker(config.Node().GetDuration(CfgGossipTipsBroadcastInterval))
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				broadcastOldestTips()
			case <-shutdownSignal:
				return
			}
		}
	}, shutdown.PriorityGossip); err != nil {
		log.Fatal("Couldn't create demon: %s", err)
	}
}

// broadcasts up to maxOldestTipsToBroadcastPerInterval tips from the tip pool
// to all connected neighbors.
func broadcastOldestTips() {
	for toBroadcast := maxOldestTipsToBroadcastPerInterval; toBroadcast >= 0; toBroadcast-- {
		msgID := tips.Next()
		if msgID == message.EmptyId {
			break
		}
		log.Debugf("broadcasting tip %s", msgID)
		broadcastMessage(msgID)
	}
}

// broadcasts the given message to all neighbors if it exists.
func broadcastMessage(msgID message.Id) {
	cachedMessage := messagelayer.Tangle().Message(msgID)
	defer cachedMessage.Release()
	if !cachedMessage.Exists() {
		return
	}
	msg := cachedMessage.Unwrap()
	Manager().SendMessage(msg.Bytes())
}
