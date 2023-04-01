package blockdag

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/stringify"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with Tangle related metadata.
type Block struct {
	missing              bool
	solid                bool
	invalid              bool
	orphaned             bool
	future               bool
	strongChildren       []*Block
	weakChildren         []*Block
	likedInsteadChildren []*Block
	mutex                sync.RWMutex

	*ModelsBlock
}

type ModelsBlock = models.Block

// NewBlock creates a new Block with the given options.
func NewBlock(data *models.Block, opts ...options.Option[Block]) (newBlock *Block) {
	return options.Apply(&Block{
		strongChildren:       make([]*Block, 0),
		weakChildren:         make([]*Block, 0),
		likedInsteadChildren: make([]*Block, 0),
		ModelsBlock:          data,
	}, opts)
}

func NewRootBlock(id models.BlockID, slotTimeProvider *slot.TimeProvider, opts ...options.Option[models.Block]) (rootBlock *Block) {
	issuingTime := time.Unix(slotTimeProvider.GenesisUnixTime(), 0)
	if id.Index() > 0 {
		issuingTime = slotTimeProvider.EndTime(id.Index())
	}
	return NewBlock(
		models.NewEmptyBlock(id, append([]options.Option[models.Block]{models.WithIssuingTime(issuingTime)}, opts...)...),
		WithSolid(true),
		WithMissing(false),
	)
}

// IsMissing returns a flag that indicates if the underlying Block data hasn't been stored, yet.
func (b *Block) IsMissing() (isMissing bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.missing
}

// IsSolid returns true if the Block is solid (the entire causal history is known).
func (b *Block) IsSolid() (isSolid bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.solid
}

// IsInvalid returns true if the Block was marked as invalid.
func (b *Block) IsInvalid() (isInvalid bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.invalid
}

// IsFuture returns true if the Block is a future Block (we haven't committed to its commitment slot yet).
func (b *Block) IsFuture() (isFuture bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.future
}

// SetFuture marks the Block as future block.
func (b *Block) SetFuture() (wasUpdated bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.future {
		return false
	}

	b.future = true
	return true
}

// IsOrphaned returns true if the Block is orphaned (either due to being marked as orphaned itself or because it has
// orphaned Blocks in its past cone).
func (b *Block) IsOrphaned() (isOrphaned bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.orphaned
}

// Children returns the children of the Block.
func (b *Block) Children() (children []*Block) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	seenBlockIDs := make(map[models.BlockID]types.Empty)
	for _, parentsByType := range [][]*Block{
		b.strongChildren,
		b.weakChildren,
		b.likedInsteadChildren,
	} {
		for _, childMetadata := range parentsByType {
			if _, exists := seenBlockIDs[childMetadata.ID()]; !exists {
				children = append(children, childMetadata)
				seenBlockIDs[childMetadata.ID()] = types.Void
			}
		}
	}

	return children
}

func (b *Block) StrongChildren() []*Block {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return lo.CopySlice(b.strongChildren)
}

func (b *Block) WeakChildren() []*Block {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return lo.CopySlice(b.weakChildren)
}

func (b *Block) LikedInsteadChildren() []*Block {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return lo.CopySlice(b.likedInsteadChildren)
}

// SetSolid marks the Block as solid.
func (b *Block) SetSolid() (wasUpdated bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if wasUpdated = !b.solid; wasUpdated {
		b.solid = true
	}

	return
}

// SetInvalid marks the Block as invalid.
func (b *Block) SetInvalid() (wasUpdated bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.invalid {
		return false
	}

	b.invalid = true

	return true
}

// SetOrphaned sets the orphaned flag of the Block.
func (b *Block) SetOrphaned(orphaned bool) (wasUpdated bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.orphaned == orphaned {
		return false
	}
	b.orphaned = orphaned

	return true
}

// AppendChild adds a child of the corresponding type to the Block.
func (b *Block) AppendChild(child *Block, childType models.ParentsType) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	switch childType {
	case models.StrongParentType:
		b.strongChildren = append(b.strongChildren, child)
	case models.WeakParentType:
		b.weakChildren = append(b.weakChildren, child)
	case models.ShallowLikeParentType:
		b.likedInsteadChildren = append(b.likedInsteadChildren, child)
	}
}

// Update publishes the given Block data to the underlying Block and marks it as no longer missing.
func (b *Block) Update(data *models.Block) (wasPublished bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if !b.missing {
		return
	}

	b.ImportStorable(&data.Storable)
	b.missing = false

	return true
}

func (b *Block) String() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	builder := stringify.NewStructBuilder("BlockDAG.Block", stringify.NewStructField("id", b.ID()))
	builder.AddField(stringify.NewStructField("Missing", b.missing))
	builder.AddField(stringify.NewStructField("Solid", b.solid))
	builder.AddField(stringify.NewStructField("Invalid", b.invalid))
	builder.AddField(stringify.NewStructField("Orphaned", b.orphaned))

	for index, child := range b.strongChildren {
		builder.AddField(stringify.NewStructField(fmt.Sprintf("strongChildren%d", index), child.ID().String()))
	}

	for index, child := range b.weakChildren {
		builder.AddField(stringify.NewStructField(fmt.Sprintf("weakChildren%d", index), child.ID().String()))
	}

	for index, child := range b.likedInsteadChildren {
		builder.AddField(stringify.NewStructField(fmt.Sprintf("likedInsteadChildren%d", index), child.ID().String()))
	}

	builder.AddField(stringify.NewStructField("ModelsBlock", b.ModelsBlock))

	return builder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithModelOptions(opts ...options.Option[models.Block]) options.Option[Block] {
	return func(block *Block) {
		options.Apply(block.ModelsBlock, opts)
	}
}

// WithMissing is a constructor Option for Blocks that initializes the given block with a specific missing flag.
func WithMissing(missing bool) options.Option[Block] {
	return func(block *Block) {
		block.missing = missing
	}
}

// WithSolid is a constructor Option for Blocks that initializes the given block with a specific solid flag.
func WithSolid(solid bool) options.Option[Block] {
	return func(block *Block) {
		block.solid = solid
	}
}

// WithOrphaned is a constructor Option for Blocks that initializes the given block with a specific orphaned flag.
func WithOrphaned(markedOrphaned bool) options.Option[Block] {
	return func(block *Block) {
		block.orphaned = markedOrphaned
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
