package tangleold

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/bytesfilter"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/typeutils"

	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/pow"
)

const (
	// MaxReattachmentTimeMin defines the max reattachment time.
	MaxReattachmentTimeMin = 10 * time.Minute
)

// region Parser ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Parser parses blocks and bytes and emits corresponding events for parsed and rejected blocks.
type Parser struct {
	bytesFilters []BytesFilter
	blockFilters []BlockFilter
	Events       *ParserEvents

	byteFiltersModified  typeutils.AtomicBool
	blockFiltersModified typeutils.AtomicBool
	bytesFiltersMutex    sync.Mutex
	blockFiltersMutex    sync.Mutex
}

// NewParser creates a new Block parser.
func NewParser() (result *Parser) {
	result = &Parser{
		bytesFilters: make([]BytesFilter, 0),
		blockFilters: make([]BlockFilter, 0),
		Events:       newParserEvents(),
	}

	// add builtin filters
	result.AddBytesFilter(NewRecentlySeenBytesFilter())
	result.AddBlockFilter(NewBlockSignatureFilter())
	result.AddBlockFilter(NewTransactionFilter())
	return
}

// Setup defines the flow of the parser.
func (p *Parser) Setup() {
	p.setupBytesFilterDataFlow()
	p.setupBlockFilterDataFlow()
}

// Parse parses the given block bytes.
func (p *Parser) Parse(blockBytes []byte, peer *peer.Peer) {
	p.bytesFilters[0].Filter(blockBytes, peer)
}

// AddBytesFilter adds the given bytes filter to the parser.
func (p *Parser) AddBytesFilter(filter BytesFilter) {
	p.bytesFiltersMutex.Lock()
	p.bytesFilters = append(p.bytesFilters, filter)
	p.bytesFiltersMutex.Unlock()
	p.byteFiltersModified.Set()
}

// AddBlockFilter adds a new block filter to the parser.
func (p *Parser) AddBlockFilter(filter BlockFilter) {
	p.blockFiltersMutex.Lock()
	p.blockFilters = append(p.blockFilters, filter)
	p.blockFiltersMutex.Unlock()
	p.blockFiltersModified.Set()
}

// sets up the byte filter data flow chain.
func (p *Parser) setupBytesFilterDataFlow() {
	if !p.byteFiltersModified.IsSet() {
		return
	}

	p.bytesFiltersMutex.Lock()
	if p.byteFiltersModified.IsSet() {
		p.byteFiltersModified.SetTo(false)

		numberOfBytesFilters := len(p.bytesFilters)
		for i := 0; i < numberOfBytesFilters; i++ {
			if i == numberOfBytesFilters-1 {
				p.bytesFilters[i].OnAccept(p.parseBlock)
			} else {
				p.bytesFilters[i].OnAccept(p.bytesFilters[i+1].Filter)
			}
			p.bytesFilters[i].OnReject(func(bytes []byte, err error, peer *peer.Peer) {
				p.Events.BytesRejected.Trigger(&BytesRejectedEvent{
					Bytes: bytes,
					Peer:  peer,
					Error: err,
				})
			})
		}
	}
	p.bytesFiltersMutex.Unlock()
}

// sets up the block filter data flow chain.
func (p *Parser) setupBlockFilterDataFlow() {
	if !p.blockFiltersModified.IsSet() {
		return
	}

	p.blockFiltersMutex.Lock()
	if p.blockFiltersModified.IsSet() {
		p.blockFiltersModified.SetTo(false)

		numberOfBlockFilters := len(p.blockFilters)
		for i := 0; i < numberOfBlockFilters; i++ {
			if i == numberOfBlockFilters-1 {
				p.blockFilters[i].OnAccept(func(blk *Block, peer *peer.Peer) {
					p.Events.BlockParsed.Trigger(&BlockParsedEvent{
						Block: blk,
						Peer:  peer,
					})
				})
			} else {
				p.blockFilters[i].OnAccept(p.blockFilters[i+1].Filter)
			}
			p.blockFilters[i].OnReject(func(blk *Block, err error, peer *peer.Peer) {
				p.Events.BlockRejected.Trigger(&BlockRejectedEvent{
					Block: blk,
					Peer:  peer,
					Error: err,
				})
			})
		}
	}
	p.blockFiltersMutex.Unlock()
}

// parses the given block and emits
func (p *Parser) parseBlock(bytes []byte, peer *peer.Peer) {
	// Validation of the block is implicitly done while decoding the bytes with serix.
	blk := new(Block)
	if _, err := serix.DefaultAPI.Decode(context.Background(), bytes, blk); err != nil {
		p.Events.BytesRejected.Trigger(&BytesRejectedEvent{
			Bytes: bytes,
			Peer:  peer,
			Error: err,
		})
	} else {
		// TODO: this could also be done by directly taking the bytes
		_ = blk.DetermineID()
		p.blockFilters[0].Filter(blk, peer)
	}
}

// Shutdown closes all the block filters.
func (p *Parser) Shutdown() {
	for _, blockFiler := range p.blockFilters {
		blockFiler.Close()
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BytesFilter //////////////////////////////////////////////////////////////////////////////////////////////////

// BytesFilter filters based on byte slices and peers.
type BytesFilter interface {
	// Filter filters up on the given bytes and peer and calls the acceptance callback
	// if the input passes or the rejection callback if the input is rejected.
	Filter(bytes []byte, peer *peer.Peer)
	// OnAccept registers the given callback as the acceptance function of the filter.
	OnAccept(callback func(bytes []byte, peer *peer.Peer))
	// OnReject registers the given callback as the rejection function of the filter.
	OnReject(callback func(bytes []byte, err error, peer *peer.Peer))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockFilter ////////////////////////////////////////////////////////////////////////////////////////////////

// BlockFilter filters based on blocks and peers.
type BlockFilter interface {
	// Filter filters up on the given block and peer and calls the acceptance callback
	// if the input passes or the rejection callback if the input is rejected.
	Filter(blk *Block, peer *peer.Peer)
	// OnAccept registers the given callback as the acceptance function of the filter.
	OnAccept(callback func(blk *Block, peer *peer.Peer))
	// OnReject registers the given callback as the rejection function of the filter.
	OnReject(callback func(blk *Block, err error, peer *peer.Peer))
	// Closer closes the filter.
	io.Closer
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockSignatureFilter ///////////////////////////////////////////////////////////////////////////////////////

// BlockSignatureFilter filters blocks based on whether their signatures are valid.
type BlockSignatureFilter struct {
	onAcceptCallback func(blk *Block, peer *peer.Peer)
	onRejectCallback func(blk *Block, err error, peer *peer.Peer)

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// NewBlockSignatureFilter creates a new block signature filter.
func NewBlockSignatureFilter() *BlockSignatureFilter {
	return new(BlockSignatureFilter)
}

// Filter filters up on the given bytes and peer and calls the acceptance callback
// if the input passes or the rejection callback if the input is rejected.
func (f *BlockSignatureFilter) Filter(blk *Block, peer *peer.Peer) {
	if valid, _ := blk.VerifySignature(); valid {
		f.getAcceptCallback()(blk, peer)
		return
	}
	f.getRejectCallback()(blk, ErrInvalidSignature, peer)
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (f *BlockSignatureFilter) OnAccept(callback func(blk *Block, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.Lock()
	f.onAcceptCallback = callback
	f.onAcceptCallbackMutex.Unlock()
}

// OnReject registers the given callback as the rejection function of the filter.
func (f *BlockSignatureFilter) OnReject(callback func(blk *Block, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.Lock()
	f.onRejectCallback = callback
	f.onRejectCallbackMutex.Unlock()
}

func (f *BlockSignatureFilter) getAcceptCallback() (result func(blk *Block, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.RLock()
	result = f.onAcceptCallback
	f.onAcceptCallbackMutex.RUnlock()
	return
}

func (f *BlockSignatureFilter) getRejectCallback() (result func(blk *Block, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.RLock()
	result = f.onRejectCallback
	f.onRejectCallbackMutex.RUnlock()
	return
}

// Close closes the filter.
func (f *BlockSignatureFilter) Close() error { return nil }

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PowFilter ////////////////////////////////////////////////////////////////////////////////////////////////////

// PowFilter is a block bytes filter validating the PoW nonce.
type PowFilter struct {
	worker     *pow.Worker
	difficulty int

	mu             sync.RWMutex
	acceptCallback func([]byte, *peer.Peer)
	rejectCallback func([]byte, error, *peer.Peer)
}

// NewPowFilter creates a new PoW bytes filter.
func NewPowFilter(worker *pow.Worker, difficulty int) *PowFilter {
	return &PowFilter{
		worker:     worker,
		difficulty: difficulty,
	}
}

// Filter checks whether the given bytes pass the PoW validation and calls the corresponding callback.
func (f *PowFilter) Filter(blkBytes []byte, p *peer.Peer) {
	if err := f.validate(blkBytes); err != nil {
		f.getRejectCallback()(blkBytes, err, p)
		return
	}
	f.getAcceptCallback()(blkBytes, p)
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (f *PowFilter) OnAccept(callback func([]byte, *peer.Peer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acceptCallback = callback
}

// OnReject registers the given callback as the rejection function of the filter.
func (f *PowFilter) OnReject(callback func([]byte, error, *peer.Peer)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejectCallback = callback
}

func (f *PowFilter) getAcceptCallback() (result func(blkBytes []byte, peer *peer.Peer)) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result = f.acceptCallback
	return
}

func (f *PowFilter) getRejectCallback() (result func(blkBytes []byte, err error, p *peer.Peer)) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result = f.rejectCallback
	return
}

func (f *PowFilter) validate(blkBytes []byte) error {
	content, err := powData(blkBytes)
	if err != nil {
		return err
	}
	zeros, err := f.worker.LeadingZeros(content)
	if err != nil {
		return err
	}
	if zeros < f.difficulty {
		return fmt.Errorf("%w: leading zeros %d for difficulty %d", ErrInvalidPOWDifficultly, zeros, f.difficulty)
	}
	return nil
}

// powData returns the bytes over which PoW should be computed.
func powData(blkBytes []byte) ([]byte, error) {
	contentLength := len(blkBytes) - ed25519.SignatureSize
	if contentLength < pow.NonceBytes {
		return nil, ErrBlockTooSmall
	}
	return blkBytes[:contentLength], nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RecentlySeenBytesFilter //////////////////////////////////////////////////////////////////////////////////////

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
func (r *RecentlySeenBytesFilter) Filter(bytes []byte, peer *peer.Peer) {
	if r.bytesFilter.Add(bytes) {
		r.getAcceptCallback()(bytes, peer)
		return
	}
	r.getRejectCallback()(bytes, ErrReceivedDuplicateBytes, peer)
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (r *RecentlySeenBytesFilter) OnAccept(callback func(bytes []byte, peer *peer.Peer)) {
	r.onAcceptCallbackMutex.Lock()
	r.onAcceptCallback = callback
	r.onAcceptCallbackMutex.Unlock()
}

// OnReject registers the given callback as the rejection function of the filter.
func (r *RecentlySeenBytesFilter) OnReject(callback func(bytes []byte, err error, peer *peer.Peer)) {
	r.onRejectCallbackMutex.Lock()
	r.onRejectCallback = callback
	r.onRejectCallbackMutex.Unlock()
}

func (r *RecentlySeenBytesFilter) getAcceptCallback() (result func(bytes []byte, peer *peer.Peer)) {
	r.onAcceptCallbackMutex.RLock()
	result = r.onAcceptCallback
	r.onAcceptCallbackMutex.RUnlock()
	return
}

func (r *RecentlySeenBytesFilter) getRejectCallback() (result func(bytes []byte, err error, peer *peer.Peer)) {
	r.onRejectCallbackMutex.RLock()
	result = r.onRejectCallback
	r.onRejectCallbackMutex.RUnlock()
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionFilter ////////////////////////////////////////////////////////////////////////////////////////////

// NewTransactionFilter creates a new transaction filter.
func NewTransactionFilter() *TransactionFilter {
	return new(TransactionFilter)
}

// TransactionFilter filters blocks based on their timestamps and transaction timestamp.
type TransactionFilter struct {
	onAcceptCallback func(blk *Block, peer *peer.Peer)
	onRejectCallback func(blk *Block, err error, peer *peer.Peer)

	onAcceptCallbackMutex sync.RWMutex
	onRejectCallbackMutex sync.RWMutex
}

// Filter compares the timestamps between the block, and it's transaction payload and calls the corresponding callback.
func (f *TransactionFilter) Filter(blk *Block, peer *peer.Peer) {
	if tx, ok := blk.Payload().(*devnetvm.Transaction); ok {
		if !isBlockAndTransactionTimestampsValid(tx, blk) {
			f.getRejectCallback()(blk, ErrInvalidBlockAndTransactionTimestamp, peer)
			return
		}
	}
	f.getAcceptCallback()(blk, peer)
}

func isBlockAndTransactionTimestampsValid(transaction *devnetvm.Transaction, block *Block) bool {
	transactionTimestamp := transaction.Essence().Timestamp()
	blockTimestamp := block.IssuingTime()
	return blockTimestamp.Sub(transactionTimestamp).Milliseconds() >= 0 && blockTimestamp.Sub(transactionTimestamp) <= MaxReattachmentTimeMin
}

// OnAccept registers the given callback as the acceptance function of the filter.
func (f *TransactionFilter) OnAccept(callback func(blk *Block, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.Lock()
	defer f.onAcceptCallbackMutex.Unlock()
	f.onAcceptCallback = callback
}

// OnReject registers the given callback as the rejection function of the filter.
func (f *TransactionFilter) OnReject(callback func(blk *Block, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.Lock()
	defer f.onRejectCallbackMutex.Unlock()
	f.onRejectCallback = callback
}

func (f *TransactionFilter) getAcceptCallback() (result func(blk *Block, peer *peer.Peer)) {
	f.onAcceptCallbackMutex.RLock()
	result = f.onAcceptCallback
	f.onAcceptCallbackMutex.RUnlock()
	return
}

func (f *TransactionFilter) getRejectCallback() (result func(blk *Block, err error, peer *peer.Peer)) {
	f.onRejectCallbackMutex.RLock()
	result = f.onRejectCallback
	f.onRejectCallbackMutex.RUnlock()
	return
}

// Close closes the filter.
func (f *TransactionFilter) Close() error { return nil }

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Errors ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// ErrInvalidPOWDifficultly is returned when the nonce of a block does not fulfill the PoW difficulty.
	ErrInvalidPOWDifficultly = errors.New("invalid PoW")

	// ErrBlockTooSmall is returned when the block does not contain enough data for the PoW.
	ErrBlockTooSmall = errors.New("block too small")

	// ErrInvalidSignature is returned when a block contains an invalid signature.
	ErrInvalidSignature = fmt.Errorf("invalid signature")

	// ErrReceivedDuplicateBytes is returned when duplicated bytes are rejected.
	ErrReceivedDuplicateBytes = fmt.Errorf("received duplicate bytes")

	// ErrInvalidBlockAndTransactionTimestamp is returned when the block its transaction timestamps are invalid.
	ErrInvalidBlockAndTransactionTimestamp = fmt.Errorf("invalid block and transaction timestamp")
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
