package builtinfilters

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/bytesfilter"
)

var ErrReceivedDuplicateBytes = fmt.Errorf("received duplicate bytes")

type RecentlySeenBytesFilter struct {
	bytesFilter      *bytesfilter.BytesFilter
	onAcceptCallback func(bytes []byte, peer *peer.Peer)
	onRejectCallback func(bytes []byte, err error, peer *peer.Peer)
	workerPool       async.WorkerPool

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

func NewRecentlySeenBytesFilter() (result *RecentlySeenBytesFilter) {
	result = &RecentlySeenBytesFilter{
		bytesFilter: bytesfilter.New(100000),
	}

	return
}

func (filter *RecentlySeenBytesFilter) Filter(bytes []byte, peer *peer.Peer) {
	filter.workerPool.Submit(func() {
		if filter.bytesFilter.Add(bytes) {
			filter.getAcceptCallback()(bytes, peer)
		} else {
			filter.getRejectCallback()(bytes, ErrReceivedDuplicateBytes, peer)
		}
	})
}

func (filter *RecentlySeenBytesFilter) OnAccept(callback func(bytes []byte, peer *peer.Peer)) {
	filter.onAcceptCallbackMutex.Lock()
	filter.onAcceptCallback = callback
	filter.onAcceptCallbackMutex.Unlock()
}

func (filter *RecentlySeenBytesFilter) OnReject(callback func(bytes []byte, err error, peer *peer.Peer)) {
	filter.onRejectCallbackMutex.Lock()
	filter.onRejectCallback = callback
	filter.onRejectCallbackMutex.Unlock()
}

func (filter *RecentlySeenBytesFilter) getAcceptCallback() (result func(bytes []byte, peer *peer.Peer)) {
	filter.onAcceptCallbackMutex.Lock()
	result = filter.onAcceptCallback
	filter.onAcceptCallbackMutex.Unlock()

	return
}

func (filter *RecentlySeenBytesFilter) getRejectCallback() (result func(bytes []byte, err error, peer *peer.Peer)) {
	filter.onRejectCallbackMutex.Lock()
	result = filter.onRejectCallback
	filter.onRejectCallbackMutex.Unlock()

	return
}

func (filter *RecentlySeenBytesFilter) Shutdown() {
	filter.workerPool.ShutdownGracefully()
}
