package builtinfilters

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/bytesfilter"
)

// ErrReceivedDuplicateBytes is returned when duplicated bytes are rejected.
var ErrReceivedDuplicateBytes = fmt.Errorf("received duplicate bytes")

// RecentlySeenBytesFilter filters so that bytes which were recently seen don't pass the filter.
type RecentlySeenBytesFilter struct {
	bytesFilter      *bytesfilter.BytesFilter
	onAcceptCallback func(bytes []byte, peer *peer.Peer)
	onRejectCallback func(bytes []byte, err error, peer *peer.Peer)

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// NewRecentlySeenBytesFilter creates a new recently seen bytes filter.
func NewRecentlySeenBytesFilter() *RecentlySeenBytesFilter {
	return &RecentlySeenBytesFilter{
		bytesFilter: bytesfilter.New(100000),
	}
}

// Filter filters up on the given bytes and peer and calls the acceptance callback
// if the input passes or the rejection callback if the input is rejected.
func (filter *RecentlySeenBytesFilter) Filter(bytes []byte, peer *peer.Peer) {
	if filter.bytesFilter.Add(bytes) {
		filter.getAcceptCallback()(bytes, peer)
		return
	}
	filter.getRejectCallback()(bytes, ErrReceivedDuplicateBytes, peer)
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (filter *RecentlySeenBytesFilter) OnAccept(callback func(bytes []byte, peer *peer.Peer)) {
	filter.onAcceptCallbackMutex.Lock()
	filter.onAcceptCallback = callback
	filter.onAcceptCallbackMutex.Unlock()
}

// OnReject registers the given callback as the rejection function of the filter.
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
