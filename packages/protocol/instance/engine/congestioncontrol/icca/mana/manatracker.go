package blocklayer

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/mana/manamodels"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

type ManaTracker struct {
	ledger          *ledger.Ledger
	baseManaVectors map[manamodels.Type]manamodels.BaseManaVector
	logger          *logger.Logger
	localID         identity.LocalIdentity
}

func New(ledgerInstance *ledger.Ledger, opts ...options.Option[ManaTracker]) (manaTracker *ManaTracker) {
	return options.Apply(&ManaTracker{
		ledger:          ledgerInstance,
		baseManaVectors: make(map[manamodels.Type]manamodels.BaseManaVector),
	}, opts, func(m *ManaTracker) {

		m.baseManaVectors[manamodels.AccessMana] = manamodels.NewBaseManaVector(manamodels.AccessMana)
		m.baseManaVectors[manamodels.ConsensusMana] = manamodels.NewBaseManaVector(manamodels.ConsensusMana)
	}, (*ManaTracker).setupEvents)
}

func (m *ManaTracker) setupEvents() {
	m.onManaVectorToUpdateClosure = event.NewClosure(func(event *manamodels.ManaVectorUpdateEvent) {
		m.baseManaVectors[manamodels.ConsensusMana].BookEpoch(event.Created, event.Spent)
	})
	m.ledger.Events.TransactionAccepted.Attach(event.NewClosure(func(event *ledger.TransactionAcceptedEvent) { m.onTransactionAccepted(event.TransactionID) }))
	// mana.Events().Revoked.Attach(onRevokeEventClosure)
}

func (m *ManaTracker) onTransactionAccepted(transactionID utxo.TransactionID) {
	m.ledger.Storage.CachedTransaction(transactionID).Consume(func(transaction utxo.Transaction) {
		// holds all info mana pkg needs for correct mana calculations from the transaction
		var txInfo *manamodels.TxInfo

		devnetTransaction := transaction.(*devnetvm.Transaction)

		// process transaction object to build txInfo
		totalAmount, inputInfos := m.gatherInputInfos(devnetTransaction.Essence().Inputs())

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

		// book in only access mana
		m.baseManaVectors[manamodels.AccessMana].Book(txInfo)
	})
}

func (m *ManaTracker) gatherInputInfos(inputs devnetvm.Inputs) (totalAmount float64, inputInfos []manamodels.InputInfo) {
	inputInfos = make([]manamodels.InputInfo, 0)
	for _, input := range inputs {
		var inputInfo manamodels.InputInfo

		outputID := input.(*devnetvm.UTXOInput).ReferencedOutputID()
		m.ledger.Storage.CachedOutput(outputID).Consume(func(o utxo.Output) {
			inputInfo.InputID = o.ID()

			// first, sum balances of the input, calculate total amount as well for later
			if amount, exists := o.(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA); exists {
				inputInfo.Amount = float64(amount)
				totalAmount += float64(amount)
			}

			// look into the transaction, we need timestamp and access & consensus pledge IDs
			m.ledger.Storage.CachedOutputMetadata(outputID).Consume(func(metadata *ledger.OutputMetadata) {
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

// GetHighestManaNodes returns the n highest type mana nodes in descending order.
// It also updates the mana values for each node.
// If n is zero, it returns all nodes.
func (m *ManaTracker) GetHighestManaNodes(manaType manamodels.Type, n uint) ([]manamodels.Issuer, time.Time, error) {
	if !m.QueryAllowed() {
		return []manamodels.Issuer{}, time.Now(), manamodels.ErrQueryNotAllowed
	}
	bmv := m.baseManaVectors[manaType]
	return bmv.GetHighestManaNodes(n)
}

// GetHighestManaNodesFraction returns the highest mana that own 'p' percent of total mana.
// It also updates the mana values for each node.
// If p is zero or greater than one, it returns all nodes.
func (m *ManaTracker) GetHighestManaNodesFraction(manaType manamodels.Type, p float64) ([]manamodels.Issuer, time.Time, error) {
	if !m.QueryAllowed() {
		return []manamodels.Issuer{}, time.Now(), manamodels.ErrQueryNotAllowed
	}
	bmv := m.baseManaVectors[manaType]
	return bmv.GetHighestManaNodesFraction(p)
}

// GetManaMap returns type mana perception of the node.
func (m *ManaTracker) GetManaMap(manaType manamodels.Type) (manamodels.IssuerMap, time.Time, error) {
	if !m.QueryAllowed() {
		return manamodels.IssuerMap{}, time.Now(), manamodels.ErrQueryNotAllowed
	}
	return m.baseManaVectors[manaType].GetManaMap()
}

// GetCMana is a wrapper for the approval weight.
func (m *ManaTracker) GetCMana() map[identity.ID]float64 {
	mana, _, err := m.GetManaMap(manamodels.ConsensusMana)
	if err != nil {
		panic(err)
	}
	return mana
}

// GetTotalMana returns sum of mana of all nodes in the network.
func (m *ManaTracker) GetTotalMana(manaType manamodels.Type) (float64, time.Time, error) {
	if !m.QueryAllowed() {
		return 0, time.Now(), manamodels.ErrQueryNotAllowed
	}
	manaMap, updateTime, err := m.baseManaVectors[manaType].GetManaMap()
	if err != nil {
		return 0, time.Now(), err
	}

	var sum float64
	for _, m := range manaMap {
		sum += m
	}
	return sum, updateTime, nil
}

// GetAccessMana returns the access mana of the node specified.
func (m *ManaTracker) GetAccessMana(nodeID identity.ID) (float64, time.Time, error) {
	if !m.QueryAllowed() {
		return 0, time.Now(), manamodels.ErrQueryNotAllowed
	}
	return m.baseManaVectors[manamodels.AccessMana].GetMana(nodeID)
}

// GetConsensusMana returns the consensus mana of the node specified.
func (m *ManaTracker) GetConsensusMana(nodeID identity.ID) (float64, time.Time, error) {
	if !m.QueryAllowed() {
		return 0, time.Now(), manamodels.ErrQueryNotAllowed
	}
	return m.baseManaVectors[manamodels.ConsensusMana].GetMana(nodeID)
}

// TODO: this should be processed on another level based on mana maps available in the manager
//// GetNeighborsMana returns the type mana of the nodes neighbors.
//func (m *ManaTracker) GetNeighborsMana(manaType manamodels.Type, neighbors []*p2p.Neighbor) (manamodels.IssuerMap, error) {
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
//}

// GetAllManaMaps returns the full mana maps for comparison with the perception of other nodes.
func (m *ManaTracker) GetAllManaMaps() (map[manamodels.Type]manamodels.IssuerMap, error) {
	if !m.QueryAllowed() {
		return make(map[manamodels.Type]manamodels.IssuerMap), manamodels.ErrQueryNotAllowed
	}
	res := make(map[manamodels.Type]manamodels.IssuerMap)
	for manaType := range m.baseManaVectors {
		res[manaType], _, _ = m.GetManaMap(manaType)
	}
	return res, nil
}

// TODO: this should be processed on another level based on mana maps available in the manager
//// GetOnlineNodes gets the list of currently known (and verified) peers in the network, and their respective mana values.
//// Sorted in descending order based on mana. Zero mana nodes are excluded.
//func (m *ManaTracker) GetOnlineNodes(manaType manamodels.Type) (onlineNodesMana []manamodels.Issuer, t time.Time, err error) {
//	if !m.QueryAllowed() {
//		return []manamodels.Issuer{}, time.Now(), manamodels.ErrQueryNotAllowed
//	}
//	if deps.Discover == nil {
//		return
//	}
//	knownPeers := deps.Discover.GetVerifiedPeers()
//	// consider ourselves as a peer in the network too
//	knownPeers = append(knownPeers, deps.Local.Peer)
//	onlineNodesMana = make([]manamodels.Issuer, 0)
//	for _, peer := range knownPeers {
//		if m.baseManaVectors[manaType].Has(peer.ID()) {
//			var peerMana float64
//			peerMana, t, err = m.baseManaVectors[manaType].GetMana(peer.ID())
//			if err != nil {
//				return nil, t, err
//			}
//			if peerMana > 0 {
//				onlineNodesMana = append(onlineNodesMana, manamodels.Issuer{ID: peer.ID(), Mana: peerMana})
//			}
//		}
//	}
//	sort.Slice(onlineNodesMana, func(i, j int) bool {
//		return onlineNodesMana[i].Mana > onlineNodesMana[j].Mana
//	})
//	return
//}

func (m *ManaTracker) cleanupManaVectors() {
	for _, vecType := range []manamodels.Type{manamodels.AccessMana, manamodels.ConsensusMana} {
		m.baseManaVectors[vecType].RemoveZeroNodes()
	}
}

// QueryAllowed returns if the mana plugin answers queries or not.
func (m *ManaTracker) QueryAllowed() (allowed bool) {
	// if debugging enabled, reply to the query
	// if debugging is not allowed, only reply when in sync
	// return deps.Tangle.Bootstrapped() || debuggingEnabled\

	// query allowed only when base mana vectors have been initialized
	return len(m.baseManaVectors) > 0
}

// AllowedPledge represents the nodes that mana is allowed to be pledged to.
type AllowedPledge struct {
	IsFilterEnabled bool
	Allowed         set.Set[identity.ID]
}

// // EventsLogs represents the events logs.
// type EventsLogs struct {
//	Pledge []*mana.PledgedEvent `json:"pledge"`
//	Revoke []*mana.RevokedEvent `json:"revoke"`
// }
