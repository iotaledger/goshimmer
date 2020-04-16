package testutil

import (
	"testing"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
)

const sequenceKey = "seq"

var messageFactoryInstance *messagefactory.MessageFactory

func MessageFactory(t *testing.T) *messagefactory.MessageFactory {
	if messageFactoryInstance == nil {
		localIdentity := identity.GenerateLocalIdentity()
		tipSelector := tipselector.New()

		t.Cleanup(func() {
			messageFactoryInstance = nil
		})

		messageFactoryInstance = messagefactory.New(DB(t), localIdentity, tipSelector, []byte(sequenceKey))
	}

	return messageFactoryInstance
}
