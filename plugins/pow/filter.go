package pow

import (
	"sync"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/autopeering/peer"
)

type powFilter struct {
	sync.Mutex

	workerPool async.WorkerPool
	onAccept   func([]byte, *peer.Peer)
	onReject   func([]byte, error, *peer.Peer)
}

func (f *powFilter) Filter(bytes []byte, peer *peer.Peer) {
	f.workerPool.Submit(func() {
		if err := ValidatePOW(bytes); err != nil {
			f.getRejectCallback()(bytes, err, peer)
			return
		}
		f.getAcceptCallback()(bytes, peer)
	})
}

func (f *powFilter) OnAccept(callback func([]byte, *peer.Peer)) {
	f.Lock()
	defer f.Unlock()
	f.onAccept = callback
}

func (f *powFilter) OnReject(callback func([]byte, error, *peer.Peer)) {
	f.Lock()
	defer f.Unlock()
	f.onReject = callback
}

func (f *powFilter) Shutdown() {
	f.workerPool.ShutdownGracefully()
}

func (f *powFilter) getAcceptCallback() func([]byte, *peer.Peer) {
	f.Lock()
	defer f.Unlock()
	return f.onAccept
}

func (f *powFilter) getRejectCallback() func([]byte, error, *peer.Peer) {
	f.Lock()
	defer f.Unlock()
	return f.onReject
}
