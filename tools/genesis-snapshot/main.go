package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	cfgGenesisTokenAmount   = "token-amount"
	cfgSnapshotFileName     = "snapshot-file"
	cfgSnapshotGenesisSeed  = "seed"
	defaultSnapshotFileName = "./snapshot.bin"

	// In the docker network tokensToPledge is also pledged to the faucet
	tokensToPledge   uint64 = 1000000000000000                               // we pledge the following amount to nodesToPledge
	peerMasterPledge        = "EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP" // peer master pledge ID
)

var nodesToPledge = []string{
	"CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3", // Faucet
}

func init() {
	flag.Uint64(cfgGenesisTokenAmount, 1000000000000000, "the amount of tokens to add to the genesis output") // we pledge this amount to peer master
	flag.String(cfgSnapshotFileName, defaultSnapshotFileName, "the name of the generated snapshot file")
	// flag.String(cfgSnapshotGenesisSeed, "", "the genesis seed")
	// Most recent seed when checking ../integration-tests/assets :
	flag.String(cfgSnapshotGenesisSeed, "7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih", "the genesis seed")
}

func main() {
	flag.Parse()
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}
	genesisTokenAmount := viper.GetUint64(cfgGenesisTokenAmount)
	snapshotFileName := viper.GetString(cfgSnapshotFileName)
	log.Printf("creating snapshot %s...", snapshotFileName)

	seedStr := viper.GetString(cfgSnapshotGenesisSeed)
	if seedStr == "" {
		log.Fatal("Seed is required. Enter it via --seed=... ")
	}
	seedBytes, err := base58.Decode(seedStr)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to decode base58 seed: %w", err))
	}
	genesisSeed := seed.NewSeed(seedBytes)

	// Wallet mocker
	mockedConnector := newMockConnector(
		&wallet.Output{
			Address: genesisSeed.Address(0),
			Object: ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
				ledgerstate.ColorIOTA: genesisTokenAmount,
			}), &ledgerstate.ED25519Address{}).SetID(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0)),
			InclusionState: wallet.InclusionState{
				Liked:     true,
				Confirmed: true,
			},
		},
	)

	// define maps for snapshot
	transactionsMap := make(map[ledgerstate.TransactionID]ledgerstate.Record)
	accessManaMap := make(map[identity.ID]ledgerstate.AccessMana)

	//////////////// prepare pledge to Peer master ////////////////////////////////////////////////////////////////
	output := ledgerstate.NewSigLockedColoredOutput(
		ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: genesisTokenAmount,
		}),
		genesisSeed.Address(0).Address(),
	)

	pubKey, err := ed25519.PublicKeyFromString(peerMasterPledge)
	if err != nil {
		panic(err)
	}
	nodeID := identity.NewID(pubKey)
	tx := ledgerstate.NewTransaction(ledgerstate.NewTransactionEssence(
		0,
		time.Unix(tangle.DefaultGenesisTime, 0),
		nodeID,
		nodeID,
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs(output),
	), ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})

	txRecord := ledgerstate.Record{
		Essence:        tx.Essence(),
		UnlockBlocks:   tx.UnlockBlocks(),
		UnspentOutputs: []bool{true},
	}
	transactionsMap[tx.ID()] = txRecord
	accessManaRecord := ledgerstate.AccessMana{
		Value:     float64(genesisTokenAmount),
		Timestamp: time.Unix(tangle.DefaultGenesisTime, 0),
	}
	accessManaMap[nodeID] = accessManaRecord

	//////////////// prepare pledge for nodesToPledge ////////////////////////////////////////////////////////////////
	randomSeed := seed.NewSeed()
	output1 := ledgerstate.NewSigLockedColoredOutput(
		ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: tokensToPledge,
		}),
		randomSeed.Address(0).Address(),
	)

	// nodesToPledge is currently the faucet, and
	for i, pk := range nodesToPledge {
		pubKey, err = ed25519.PublicKeyFromString(pk)
		if err != nil {
			panic(err)
		}
		nodeID = identity.NewID(pubKey)

		tx = ledgerstate.NewTransaction(ledgerstate.NewTransactionEssence(
			0,
			time.Unix(tangle.DefaultGenesisTime, 0),
			nodeID,
			nodeID,
			ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, uint16(i+1)))),
			ledgerstate.NewOutputs(output1),
		), ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})

		record := ledgerstate.Record{
			Essence:        tx.Essence(),
			UnlockBlocks:   tx.UnlockBlocks(),
			UnspentOutputs: []bool{true},
		}
		transactionsMap[tx.ID()] = record
		accessManaRecord := ledgerstate.AccessMana{
			Value:     float64(tokensToPledge),
			Timestamp: time.Unix(tangle.DefaultGenesisTime, 0),
		}
		accessManaMap[nodeID] = accessManaRecord
	}

	newSnapshot := &ledgerstate.Snapshot{
		AccessManaByNode: accessManaMap,
		Transactions:     transactionsMap,
	}

	genesisWallet := wallet.New(wallet.Import(genesisSeed, 1, []bitmask.BitMask{}, wallet.NewAssetRegistry("test")), wallet.GenericConnector(mockedConnector))
	genesisAddress := genesisWallet.Seed().Address(0).Address()

	log.Println("genesis:")
	log.Printf("-> seed (base58): %s", genesisWallet.Seed().String())
	log.Printf("-> output address (base58): %s", genesisAddress.Base58())
	log.Printf("-> output id (base58): %s", ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))
	log.Printf("-> token amount: %d", genesisTokenAmount)

	f, err := os.OpenFile(snapshotFileName, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal("unable to create snapshot file", err)
	}

	n, err := newSnapshot.WriteTo(f)
	if err != nil {
		log.Fatal("unable to write snapshot content to file", err)
	}

	log.Printf("Bytes written %d", n)
	f.Close()

	log.Printf("created %s, bye", snapshotFileName)

	f, err = os.OpenFile(snapshotFileName, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal("unable to create snapshot file ", err)
	}

	readSnapshot := &ledgerstate.Snapshot{}
	_, err = readSnapshot.ReadFrom(f)
	if err != nil {
		log.Fatal("unable to read snapshot file ", err)
	}
	f.Close()

	fmt.Println("\n================= read Snapshot ===============")
	fmt.Printf("\n================= %d Snapshot Txs ===============\n", len(readSnapshot.Transactions))
	for key, txRecord := range readSnapshot.Transactions {
		fmt.Println("===== key =", key)
		fmt.Println(txRecord)
	}
	fmt.Printf("\n================= %d Snapshot Access Manas ===============\n", len(readSnapshot.AccessManaByNode))
	for key, accessManaNode := range readSnapshot.AccessManaByNode {
		fmt.Println("===== key =", key)
		fmt.Println(accessManaNode)
	}
}

type mockConnector struct {
	outputs map[address.Address]map[ledgerstate.OutputID]*wallet.Output
}

func (connector *mockConnector) UnspentOutputs(addresses ...address.Address) (outputs wallet.OutputsByAddressAndOutputID, err error) {
	outputs = make(wallet.OutputsByAddressAndOutputID)
	for _, addr := range addresses {
		for outputID, output := range connector.outputs[addr] {
			if !output.InclusionState.Spent {
				if _, outputsExist := outputs[addr]; !outputsExist {
					outputs[addr] = make(map[ledgerstate.OutputID]*wallet.Output)
				}

				outputs[addr][outputID] = output
			}
		}
	}

	return
}

func newMockConnector(outputs ...*wallet.Output) (connector *mockConnector) {
	connector = &mockConnector{
		outputs: make(map[address.Address]map[ledgerstate.OutputID]*wallet.Output),
	}

	for _, output := range outputs {
		if _, addressExists := connector.outputs[output.Address]; !addressExists {
			connector.outputs[output.Address] = make(map[ledgerstate.OutputID]*wallet.Output)
		}

		connector.outputs[output.Address][output.Object.ID()] = output
	}

	return
}

func (connector *mockConnector) RequestFaucetFunds(addr address.Address, powTarget int) (err error) {
	// generate random transaction id
	return
}

func (connector *mockConnector) SendTransaction(tx *ledgerstate.Transaction) (err error) {
	// mark outputs as spent
	return
}

func (connector *mockConnector) GetAllowedPledgeIDs() (pledgeIDMap map[mana.Type][]string, err error) {
	return
}

func (connector *mockConnector) GetTransactionInclusionState(txID ledgerstate.TransactionID) (inc ledgerstate.InclusionState, err error) {
	return
}

func (connector *mockConnector) GetUnspentAliasOutput(addr *ledgerstate.AliasAddress) (output *ledgerstate.AliasOutput, err error) {
	return
}
