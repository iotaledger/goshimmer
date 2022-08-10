package chat

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(Payload{}, serix.TypeSettings{}.WithObjectType(uint32(new(Payload).Type())))
	if err != nil {
		panic(fmt.Errorf("error registering Chat type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterInterfaceObjects((*payload.Payload)(nil), new(Payload))
	if err != nil {
		panic(fmt.Errorf("error registering Chat as Payload interface: %w", err))
	}
}

// NewChat creates a new Chat.
func NewChat() *Chat {
	return &Chat{
		Events: newEvents(),
	}
}

// Chat manages chats happening over the Tangle.
type Chat struct {
	*Events
}

const (
	// PayloadName defines the name of the chat payload.
	PayloadName = "chat"
	payloadType = 989
)

// Payload represents the chat payload type.
type Payload struct {
	model.Immutable[Payload, *Payload, payloadModel] `serix:"0"`
}

type payloadModel struct {
	From  string `serix:"0,lengthPrefixType=uint32"`
	To    string `serix:"1,lengthPrefixType=uint32"`
	Block string `serix:"2,lengthPrefixType=uint32"`
}

// NewPayload creates a new chat payload.
func NewPayload(from, to, block string) *Payload {
	return model.NewImmutable[Payload](&payloadModel{
		From:  from,
		To:    to,
		Block: block,
	},
	)
}

// Type represents the identifier which addresses the chat payload type.
var Type = payload.NewType(payloadType, PayloadName)

// Type returns the type of the Payload.
func (p *Payload) Type() payload.Type {
	return Type
}

// From returns an author of the block.
func (p *Payload) From() string {
	return p.M.From
}

// To returns a recipient of the block.
func (p *Payload) To() string {
	return p.M.To
}

// Block returns the block contents.
func (p *Payload) Block() string {
	return p.M.Block
}
