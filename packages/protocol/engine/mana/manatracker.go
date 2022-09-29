package mana

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana/manamodels"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

const EpochDelay = 2

type Tracker struct {
	ledger          *ledger.Ledger
	baseManaVectors map[manamodels.Type]*manamodels.ManaBaseVector

	OnManaVectorToUpdateClosure *event.Closure[*ManaVectorUpdateEvent]
	Events                      *Events
	cManaTargetEpoch            epoch.Index
}

func NewTracker(ledgerInstance *ledger.Ledger, opts ...options.Option[Tracker]) (manaTracker *Tracker) {
	return options.Apply(&Tracker{
		Events:          NewEvents(),
		ledger:          ledgerInstance,
		baseManaVectors: make(map[manamodels.Type]*manamodels.ManaBaseVector),
	}, opts, func(m *Tracker) {

		m.baseManaVectors[manamodels.AccessMana] = manamodels.NewManaBaseVector(manamodels.AccessMana)
		m.baseManaVectors[manamodels.ConsensusMana] = manamodels.NewManaBaseVector(manamodels.ConsensusMana)
	}, (*Tracker).setupEvents)
}

func (t *Tracker) setupEvents() {
	t.OnManaVectorToUpdateClosure = event.NewClosure(func(event *ManaVectorUpdateEvent) {
		t.BookEpoch(event.Created, event.Spent)
	})
	t.ledger.Events.TransactionAccepted.Attach(event.NewClosure(func(event *ledger.TransactionAcceptedEvent) { t.onTransactionAccepted(event.TransactionID) }))
	// mana.Events().Revoked.Attach(onRevokeEventClosure)
}

func (t *Tracker) onTransactionAccepted(transactionID utxo.TransactionID) {
	t.ledger.Storage.CachedTransaction(transactionID).Consume(func(transaction utxo.Transaction) {
		// holds all info mana pkg needs for correct mana calculations from the transaction
		var txInfo *manamodels.TxInfo

		devnetTransaction := transaction.(*devnetvm.Transaction)

		// process transaction object to build txInfo
		totalAmount, inputInfos := t.gatherInputInfos(devnetTransaction.Essence().Inputs())

		txInfo = &manamodels.TxInfo{
			TimeStamp:     devnetTransaction.Essence().Timestamp(),
			TransactionID: transactionID,
			TotalBalance:  totalAmount,
			PledgeID: map[manamodels.Type]identity.ID{
				manamodels.AccessMana:    devnetTransaction.Essence().AccessPledgeID(),
				manamodels.ConsensusMana: devnetTransaction.Essence().ConsensusPledgeID(),
			},
			InputInfos: inputInfos,
		}

		// bookTransaction in only access mana
		t.BookTransaction(txInfo)
	})
}

func (t *Tracker) gatherInputInfos(inputs devnetvm.Inputs) (totalAmount int64, inputInfos []manamodels.InputInfo) {
	inputInfos = make([]manamodels.InputInfo, 0)
	for _, input := range inputs {
		var inputInfo manamodels.InputInfo

		outputID := input.(*devnetvm.UTXOInput).ReferencedOutputID()
		t.ledger.Storage.CachedOutput(outputID).Consume(func(o utxo.Output) {
			inputInfo.InputID = o.ID()

			// first, sum balances of the input, calculate total amount as well for later
			if amount, exists := o.(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA); exists {
				inputInfo.Amount = int64(amount)
				totalAmount += int64(amount)
			}

			// look into the transaction, we need timestamp and access & consensus pledge IDs
			t.ledger.Storage.CachedOutputMetadata(outputID).Consume(func(metadata *ledger.OutputMetadata) {
				inputInfo.PledgeID = map[manamodels.Type]identity.ID{
					manamodels.AccessMana:    metadata.AccessManaPledgeID(),
					manamodels.ConsensusMana: metadata.ConsensusManaPledgeID(),
				}
			})
		})
		inputInfos = append(inputInfos, inputInfo)
	}
	return totalAmount, inputInfos
}

// BookTransaction books mana for a transaction.
func (t *Tracker) BookTransaction(txInfo *manamodels.TxInfo) {
	revokeEvents, pledgeEvents, updateEvents := t.bookTransaction(txInfo)

	t.triggerManaEvents(revokeEvents, pledgeEvents, updateEvents)
}
func (t *Tracker) bookTransaction(txInfo *manamodels.TxInfo) (revokeEvents []*RevokedEvent, pledgeEvents []*PledgedEvent, updateEvents []*UpdatedEvent) {
	accessManaVector := t.baseManaVectors[manamodels.AccessMana]

	accessManaVector.Lock()
	defer accessManaVector.Unlock()
	// first, revoke mana from previous owners
	for _, inputInfo := range txInfo.InputInfos {
		// which issuer did the input pledge mana to?
		oldPledgeIssuerID := inputInfo.PledgeID[manamodels.AccessMana]
		oldMana := accessManaVector.GetOldManaAndRevoke(oldPledgeIssuerID, inputInfo.Amount)
		// save events for later triggering
		revokeEvents = append(revokeEvents, &RevokedEvent{
			IssuerID:      oldPledgeIssuerID,
			Amount:        inputInfo.Amount,
			Time:          txInfo.TimeStamp,
			ManaType:      manamodels.AccessMana,
			TransactionID: txInfo.TransactionID,
			InputID:       inputInfo.InputID,
		})
		updateEvents = append(updateEvents, &UpdatedEvent{
			IssuerID: oldPledgeIssuerID,
			OldMana:  &oldMana,
			NewMana:  accessManaVector.M.Vector[oldPledgeIssuerID],
			ManaType: manamodels.AccessMana,
		})
	}
	// second, pledge mana to new issuers
	newPledgeIssuerID := txInfo.PledgeID[manamodels.AccessMana]
	oldMana := accessManaVector.GetOldManaAndPledge(newPledgeIssuerID, txInfo.TotalBalance)

	pledgeEvents = append(pledgeEvents, &PledgedEvent{
		IssuerID:      newPledgeIssuerID,
		Amount:        txInfo.SumInputs(),
		Time:          txInfo.TimeStamp,
		ManaType:      manamodels.AccessMana,
		TransactionID: txInfo.TransactionID,
	})
	updateEvents = append(updateEvents, &UpdatedEvent{
		IssuerID: newPledgeIssuerID,
		OldMana:  &oldMana,
		NewMana:  accessManaVector.M.Vector[newPledgeIssuerID],
		ManaType: manamodels.AccessMana,
	})

	return revokeEvents, pledgeEvents, updateEvents
}

// BookEpoch takes care of the booking of consensus mana for the given committed epoch.
func (t *Tracker) BookEpoch(created, spent []*ledger.OutputWithMetadata) {
	revokeEvents, pledgeEvents, updateEvents := t.bookEpoch(created, spent)
	t.triggerManaEvents(revokeEvents, pledgeEvents, updateEvents)
}

func (t *Tracker) bookEpoch(created, spent []*ledger.OutputWithMetadata) (revokeEvents []*RevokedEvent, pledgeEvents []*PledgedEvent, updateEvents []*UpdatedEvent) {
	consensusManaVector := t.baseManaVectors[manamodels.ConsensusMana]
	consensusManaVector.Lock()
	defer consensusManaVector.Unlock()

	// first, revoke mana from previous owners
	for _, output := range spent {
		idToRevoke := consensusManaVector.GetIDBasedOnManaType(output)
		outputIOTAs, exists := output.Output().(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA)
		if !exists {
			continue
		}
		oldMana := consensusManaVector.GetOldManaAndRevoke(idToRevoke, int64(outputIOTAs))

		// save events for later triggering
		revokeEvents = append(revokeEvents, &RevokedEvent{
			IssuerID:      idToRevoke,
			Amount:        int64(outputIOTAs),
			Time:          output.CreationTime(),
			ManaType:      manamodels.ConsensusMana,
			TransactionID: output.ID().TransactionID,
			InputID:       output.ID(),
		})
		updateEvents = append(updateEvents, &UpdatedEvent{
			IssuerID: idToRevoke,
			OldMana:  &oldMana,
			NewMana:  consensusManaVector.M.Vector[idToRevoke],
			ManaType: manamodels.ConsensusMana,
		})
	}
	// second, pledge mana to new issuers
	for _, output := range created {
		idToPledge := consensusManaVector.GetIDBasedOnManaType(output)

		outputIOTAs, exists := output.Output().(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA)
		if !exists {
			continue
		}
		oldMana := consensusManaVector.GetOldManaAndPledge(idToPledge, int64(outputIOTAs))
		pledgeEvents = append(pledgeEvents, &PledgedEvent{
			IssuerID:      idToPledge,
			Amount:        int64(outputIOTAs),
			Time:          output.CreationTime(),
			ManaType:      manamodels.ConsensusMana,
			TransactionID: output.Output().ID().TransactionID,
		})

		updateEvents = append(updateEvents, &UpdatedEvent{
			IssuerID: idToPledge,
			OldMana:  &oldMana,
			NewMana:  consensusManaVector.M.Vector[idToPledge],
			ManaType: manamodels.ConsensusMana,
		})
	}

	return revokeEvents, pledgeEvents, updateEvents
}

func (t *Tracker) triggerManaEvents(revokeEvents []*RevokedEvent, pledgeEvents []*PledgedEvent, updateEvents []*UpdatedEvent) {
	// trigger the events once we released the lock on the mana vector
	for _, ev := range revokeEvents {
		t.Events.Revoked.Trigger(ev)
	}
	for _, ev := range pledgeEvents {
		t.Events.Pledged.Trigger(ev)
	}
	for _, ev := range updateEvents {
		t.Events.Updated.Trigger(ev)
	}
}

// GetHighestManaIssuers returns the n highest type mana issuers in descending order.
// It also updates the mana values for each issuer.
// If n is zero, it returns all issuers.
func (t *Tracker) GetHighestManaIssuers(manaType manamodels.Type, n uint) ([]manamodels.Issuer, time.Time, error) {
	if !t.QueryAllowed() {
		return []manamodels.Issuer{}, time.Now(), manamodels.ErrQueryNotAllowed
	}
	bmv := t.baseManaVectors[manaType]
	return bmv.GetHighestManaIssuers(n)
}

// GetHighestManaIssuersFraction returns the highest mana that own 'p' percent of total mana.
// It also updates the mana values for each issuer.
// If p is zero or greater than one, it returns all issuers.
func (t *Tracker) GetHighestManaIssuersFraction(manaType manamodels.Type, p float64) ([]manamodels.Issuer, time.Time, error) {
	if !t.QueryAllowed() {
		return []manamodels.Issuer{}, time.Now(), manamodels.ErrQueryNotAllowed
	}
	bmv := t.baseManaVectors[manaType]
	return bmv.GetHighestManaIssuersFraction(p)
}

// GetManaMap returns type mana perception of the issuer.
func (t *Tracker) GetManaMap(manaType manamodels.Type) (manamodels.IssuerMap, time.Time, error) {
	if !t.QueryAllowed() {
		return manamodels.IssuerMap{}, time.Now(), manamodels.ErrQueryNotAllowed
	}
	manaBaseVector := t.baseManaVectors[manaType]
	return manaBaseVector.GetManaMap()
}

// GetCMana is a wrapper for the approval weight.
func (t *Tracker) GetCMana() map[identity.ID]int64 {
	mana, _, err := t.GetManaMap(manamodels.ConsensusMana)
	if err != nil {
		panic(err)
	}
	return mana
}

// GetTotalMana returns sum of mana of all issuers in the network.
func (t *Tracker) GetTotalMana(manaType manamodels.Type) (int64, time.Time, error) {
	if !t.QueryAllowed() {
		return 0, time.Now(), manamodels.ErrQueryNotAllowed
	}
	manaBaseVector := t.baseManaVectors[manaType]
	manaMap, updateTime, err := manaBaseVector.GetManaMap()
	if err != nil {
		return 0, time.Now(), err
	}

	var sum int64
	for _, m := range manaMap {
		sum += m
	}
	return sum, updateTime, nil
}

// GetAccessMana returns the access mana of the issuer specified.
func (t *Tracker) GetAccessMana(issuerID identity.ID) (int64, time.Time, error) {
	if !t.QueryAllowed() {
		return 0, time.Now(), manamodels.ErrQueryNotAllowed
	}
	accessManaVector := t.baseManaVectors[manamodels.AccessMana]
	return accessManaVector.GetMana(issuerID)
}

// GetConsensusMana returns the consensus mana of the issuer specified.
func (t *Tracker) GetConsensusMana(issuerID identity.ID) (int64, time.Time, error) {
	if !t.QueryAllowed() {
		return 0, time.Now(), manamodels.ErrQueryNotAllowed
	}
	consensusManaVector := t.baseManaVectors[manamodels.ConsensusMana]
	return consensusManaVector.GetMana(issuerID)
}

// TODO: this should be processed on another level based on mana maps available in the manager
// // GetNeighborsMana returns the type mana of the issuers neighbors.
// func (m *Tracker) GetNeighborsMana(manaType manamodels.Type, neighbors []*p2p.Neighbor) (manamodels.IssuerMap, error) {
//	if !m.QueryAllowed() {
//		return manamodels.IssuerMap{}, manamodels.ErrQueryNotAllowed
//	}
//
//	res := make(manamodels.IssuerMap)
//	for _, n := range neighbors {
//		// in case of error, value is 0.0
//		value, _, _ := m.baseManaVectors[manaType].GetMana(n.ID())
//		res[n.ID()] = value
//	}
//	return res, nil
// }

// GetAllManaMaps returns the full mana maps for comparison with the perception of other issuers.
func (t *Tracker) GetAllManaMaps() (map[manamodels.Type]manamodels.IssuerMap, error) {
	if !t.QueryAllowed() {
		return make(map[manamodels.Type]manamodels.IssuerMap), manamodels.ErrQueryNotAllowed
	}
	res := make(map[manamodels.Type]manamodels.IssuerMap)
	for manaType := range t.baseManaVectors {
		res[manaType], _, _ = t.GetManaMap(manaType)
	}
	return res, nil
}

// TODO: this should be processed on another level based on mana maps available in the manager
// // GetOnlineIssuers gets the list of currently known (and verified) peers in the network, and their respective mana values.
// // Sorted in descending order based on mana. Zero mana issuers are excluded.
// func (m *Tracker) GetOnlineIssuers(manaType manamodels.Type) (onlineIssuersMana []manamodels.Issuer, t time.Time, err error) {
//	if !m.QueryAllowed() {
//		return []manamodels.Issuer{}, time.Now(), manamodels.ErrQueryNotAllowed
//	}
//	if deps.Discover == nil {
//		return
//	}
//	knownPeers := deps.Discover.GetVerifiedPeers()
//	// consider ourselves as a peer in the network too
//	knownPeers = append(knownPeers, deps.Local.Peer)
//	onlineIssuersMana = make([]manamodels.Issuer, 0)
//	for _, peer := range knownPeers {
//		if m.baseManaVectors[manaType].Has(peer.ID()) {
//			var peerMana float64
//			peerMana, t, err = m.baseManaVectors[manaType].GetMana(peer.ID())
//			if err != nil {
//				return nil, t, err
//			}
//			if peerMana > 0 {
//				onlineIssuersMana = append(onlineIssuersMana, manamodels.Issuer{ID: peer.ID(), Mana: peerMana})
//			}
//		}
//	}
//	sort.Slice(onlineIssuersMana, func(i, j int) bool {
//		return onlineIssuersMana[i].Mana > onlineIssuersMana[j].Mana
//	})
//	return
// }

func (t *Tracker) processOutputs(outputsWithMetadata []*ledger.OutputWithMetadata, manaType manamodels.Type, areCreated bool) {
	for _, outputWithMetadata := range outputsWithMetadata {
		devnetOutput := outputWithMetadata.Output().(devnetvm.Output)
		balance, exists := devnetOutput.Balances().Get(devnetvm.ColorIOTA)
		// TODO: shouldn't it get all balances of all colored coins instead of only IOTA?
		if !exists {
			continue
		}

		baseVector := t.baseManaVectors[manaType]

		var pledgeID identity.ID
		switch manaType {
		case manamodels.AccessMana:
			pledgeID = outputWithMetadata.AccessManaPledgeID()
		case manamodels.ConsensusMana:
			pledgeID = outputWithMetadata.ConsensusManaPledgeID()
		default:
			panic("invalid mana type")
		}

		existingMana, _, err := baseVector.GetMana(pledgeID)
		if !errors.Is(err, manamodels.ErrIssuerNotFoundInBaseManaVector) {
			continue
		}
		if areCreated {
			existingMana += int64(balance)
		} else {
			existingMana -= int64(balance)
		}
		baseVector.SetMana(pledgeID, manamodels.NewManaBase(existingMana))
	}
}

func (t *Tracker) LoadSnapshotHeader(header *ledger.SnapshotHeader) {
	t.cManaTargetEpoch = header.DiffEpochIndex - epoch.Index(EpochDelay)
	if t.cManaTargetEpoch < 0 {
		t.cManaTargetEpoch = 0
	}
}

func (t *Tracker) LoadOutputsWithMetadata(outputsWithMetadata []*ledger.OutputWithMetadata) {
	t.processOutputs(outputsWithMetadata, manamodels.ConsensusMana, true)
	t.processOutputs(outputsWithMetadata, manamodels.AccessMana, true)
}

func (t *Tracker) LoadEpochDiff(diff *ledger.EpochDiff) {
	// We fix the cMana vector a few epochs in the past with respect of the latest epoch in the snapshot.
	if diff.Index() <= t.cManaTargetEpoch {
		t.processOutputs(diff.Created(), manamodels.ConsensusMana, true)
		t.processOutputs(diff.Spent(), manamodels.ConsensusMana, false)
	}

	// Only the aMana will be loaded until the latest snapshot's epoch
	t.processOutputs(diff.Created(), manamodels.AccessMana, true)
	t.processOutputs(diff.Spent(), manamodels.AccessMana, false)
}

func (t *Tracker) cleanupManaVectors() {
	for _, vecType := range []manamodels.Type{manamodels.AccessMana, manamodels.ConsensusMana} {
		manaBaseVector := t.baseManaVectors[vecType]
		manaBaseVector.RemoveZeroIssuers()
	}
}

// QueryAllowed returns if the mana plugin answers queries or not.
func (t *Tracker) QueryAllowed() (allowed bool) {
	// if debugging enabled, reply to the query
	// if debugging is not allowed, only reply when in sync
	// return deps.Tangle.Bootstrapped() || debuggingEnabled\

	// query allowed only when base mana vectors have been initialized
	return len(t.baseManaVectors) > 0
}

// AllowedPledge represents the issuers that mana is allowed to be pledged to.
type AllowedPledge struct {
	IsFilterEnabled bool
	Allowed         set.Set[identity.ID]
}
