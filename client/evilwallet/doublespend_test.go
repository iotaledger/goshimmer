package evilwallet

import (
	"fmt"
	"testing"
)

func TestDoubleSpend(t *testing.T) {
	evilwallet := NewEvilWallet()

	wallet := evilwallet.NewWallet(fresh)
	clients := evilwallet.GetClients(2)

	err := evilwallet.RequestFundsFromFaucet(wallet.Address().Base58(), WithOutputAlias("1"))
	if err != nil {
		fmt.Println(err)
		return
	}

	txA, err := evilwallet.CreateTransaction("A", WithInputs("1"), WithOutput("2", 1000000), WithIssuer(wallet))
	if err != nil {
		fmt.Println(err)
	}
	txB, err := evilwallet.CreateTransaction("B", WithInputs("1"), WithOutput("3", 1000000), WithIssuer(wallet))
	if err != nil {
		fmt.Println(err)
	}

	clients[0].PostTransaction(txA.Bytes())
	clients[1].PostTransaction(txB.Bytes())

	evilwallet.ClearAliases()
	//EvilWallet.ConflictManager.AddConflict(WithConflictID("1"), WithConflictMembers("2", "3"))
}
