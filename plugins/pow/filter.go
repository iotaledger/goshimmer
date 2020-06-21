package pow

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/autopeering/peer"
)

type powFilter struct {
	sync.Mutex

	onAccept   func(*message.Message, *peer.Peer)
	onReject   func(*message.Message, error, *peer.Peer)
	workerPool async.WorkerPool
}

func (f *powFilter) Filter(msg *message.Message, peer *peer.Peer) {
	f.workerPool.Submit(func() {
		if err := ValidatePOW(msg); err != nil {
			f.getRejectCallback()(msg, err, peer)
			return
		}
		f.getAcceptCallback()(msg, peer)
	})
}

func (f *powFilter) OnAccept(callback func(*message.Message, *peer.Peer)) {
	f.Lock()
	defer f.Unlock()
	f.onAccept = callback
}

func (f *powFilter) OnReject(callback func(*message.Message, error, *peer.Peer)) {
	f.Lock()
	defer f.Unlock()
	f.onReject = callback
}

func (f *powFilter) Shutdown() {
	f.workerPool.ShutdownGracefully()
}

func (f *powFilter) getAcceptCallback() func(*message.Message, *peer.Peer) {
	f.Lock()
	defer f.Unlock()
	return f.onAccept
}

func (f *powFilter) getRejectCallback() func(*message.Message, error, *peer.Peer) {
	f.Lock()
	defer f.Unlock()
	return f.onReject
}
