package manaverse

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

func Test(t *testing.T) {
	identity1KeyPair := ed25519.GenerateKeyPair()
	identity1 := identity.New(identity1KeyPair.PublicKey)

	manaLedger := NewMockedManaLedger()
	manaLedger.IncreaseMana(identity1.ID(), 100)

	testTangle := tangle.NewTestTangle()
	testFramework := tangle.NewMessageTestFramework(testTangle)
	testFramework.CreateMessage("A", tangle.WithStrongParents("Genesis"), tangle.WithIssuer(identity1.PublicKey()))
	testFramework.CreateMessage("B", tangle.WithStrongParents("A"), tangle.WithIssuer(identity1.PublicKey()))
	testFramework.CreateMessage("C", tangle.WithStrongParents("B"), tangle.WithIssuer(identity1.PublicKey()))

	scheduler := NewScheduler(manaLedger)
	scheduler.Events.BlockScheduled.Attach(event.NewClosure(func(block *tangle.Message) {
		fmt.Println(time.Now(), "SCHEDULED", block.ID())
	}))

	scheduler.Push(testFramework.Message("A"))
	scheduler.Push(testFramework.Message("B"))
	scheduler.Push(testFramework.Message("C"))

	time.Sleep(20 * time.Second)
}
