package blockdag

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/stringify"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Block ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Block represents a Block annotated with Tangle related metadata.
type Block struct {
	missing              bool
	solid                bool
	invalid              bool
	orphaned             bool
	strongChildren       []*Block
	weakChildren         []*Block
	likedInsteadChildren []*Block

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

func NewRootBlock(id models.BlockID, opts ...options.Option[models.Block]) (rootBlock *Block) {
	issuingTime := time.Unix(epoch.GenesisTime, 0)
	if id.Index() > 0 {
		issuingTime = id.Index().EndTime()
	}
	return NewBlock(
		models.NewEmptyBlock(id, append([]options.Option[models.Block]{models.WithIssuingTime(issuingTime)}, opts...)...),
		WithSolid(true),
		WithMissing(false),
	)
}

// IsMissing returns a flag that indicates if the underlying Block data hasn't been stored, yet.
func (b *Block) IsMissing() (isMissing bool) {
	b.RLock()
	defer b.RUnlock()

	return b.missing
}

// IsSolid returns true if the Block is solid (the entire causal history is known).
func (b *Block) IsSolid() (isSolid bool) {
	b.RLock()
	defer b.RUnlock()

	return b.solid
}

// IsInvalid returns true if the Block was marked as invalid.
func (b *Block) IsInvalid() (isInvalid bool) {
	b.RLock()
	defer b.RUnlock()

	return b.invalid
}

// IsOrphaned returns true if the Block is orphaned (either due to being marked as orphaned itself or because it has
// orphaned Blocks in its past cone).
func (b *Block) IsOrphaned() (isOrphaned bool) {
	b.RLock()
	defer b.RUnlock()

	return b.orphaned
}

// Children returns the children of the Block.
func (b *Block) Children() (children []*Block) {
	b.RLock()
	defer b.RUnlock()

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
	b.RLock()
	defer b.RUnlock()
	return lo.CopySlice(b.strongChildren)
}

func (b *Block) WeakChildren() []*Block {
	b.RLock()
	defer b.RUnlock()
	return lo.CopySlice(b.weakChildren)
}

func (b *Block) LikedInsteadChildren() []*Block {
	b.RLock()
	defer b.RUnlock()
	return lo.CopySlice(b.likedInsteadChildren)
}

// setSolid marks the Block as solid.
func (b *Block) setSolid() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if wasUpdated = !b.solid; wasUpdated {
		b.solid = true
	}

	return
}

// setInvalid marks the Block as invalid.
func (b *Block) setInvalid() (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if b.invalid {
		return false
	}

	b.invalid = true

	return true
}

func (b *Block) isOrphaned() (isOrphaned bool) {
	b.RLock()
	defer b.RUnlock()

	return b.orphaned
}

// setOrphaned sets the orphaned flag of the Block.
func (b *Block) setOrphaned(orphaned bool) (wasUpdated bool) {
	b.Lock()
	defer b.Unlock()

	if b.orphaned == orphaned {
		return false
	}
	b.orphaned = orphaned

	return true
}

// appendChild adds a child of the corresponding type to the Block.
func (b *Block) appendChild(child *Block, childType models.ParentsType) {
	b.Lock()
	defer b.Unlock()

	switch childType {
	case models.StrongParentType:
		b.strongChildren = append(b.strongChildren, child)
	case models.WeakParentType:
		b.weakChildren = append(b.weakChildren, child)
	case models.ShallowLikeParentType:
		b.likedInsteadChildren = append(b.likedInsteadChildren, child)
	}
}

// update publishes the given Block data to the underlying Block and marks it as no longer missing.
func (b *Block) update(data *models.Block) (wasPublished bool) {
	b.Lock()
	defer b.Unlock()

	if !b.missing {
		return
	}

	b.missing = false
	b.M = data.M
	b.SetSize(data.Size())

	return true
}

func (b *Block) String() string {
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
