package mana

import (
	"time"

	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana/manamodels"
)

// GetHighestManaIssuers returns the n highest type mana issuers in descending order.
// It also updates the mana values for each issuer.
// If n is zero, it returns all issuers.
func (t *Tracker) GetHighestManaIssuers(manaType manamodels.Type, n uint) ([]manamodels.Issuer, time.Time, error) {
	if !t.QueryAllowed() {
		return []manamodels.Issuer{}, time.Now(), manamodels.ErrQueryNotAllowed
	}

	return t.vectorByType(manaType).GetHighestManaIssuers(n)
}

// GetHighestManaIssuersFraction returns the highest mana that own 'p' percent of total mana.
// It also updates the mana values for each issuer.
// If p is zero or greater than one, it returns all issuers.
func (t *Tracker) GetHighestManaIssuersFraction(manaType manamodels.Type, p float64) ([]manamodels.Issuer, time.Time, error) {
	if !t.QueryAllowed() {
		return []manamodels.Issuer{}, time.Now(), manamodels.ErrQueryNotAllowed
	}

	return t.vectorByType(manaType).GetHighestManaIssuersFraction(p)
}

// GetManaMap returns type mana perception of the issuer.
func (t *Tracker) GetManaMap(manaType manamodels.Type) (manamodels.IssuerMap, time.Time, error) {
	if !t.QueryAllowed() {
		return manamodels.IssuerMap{}, time.Now(), manamodels.ErrQueryNotAllowed
	}

	return t.vectorByType(manaType).GetManaMap()
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

	manaMap, updateTime, err := t.vectorByType(manaType).GetManaMap()
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

	return t.vectorByType(manamodels.AccessMana).GetMana(issuerID)
}

// GetConsensusMana returns the consensus mana of the issuer specified.
func (t *Tracker) GetConsensusMana(issuerID identity.ID) (int64, time.Time, error) {
	if !t.QueryAllowed() {
		return 0, time.Now(), manamodels.ErrQueryNotAllowed
	}

	return t.vectorByType(manamodels.ConsensusMana).GetMana(issuerID)
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
	for _, manaType := range []manamodels.Type{manamodels.AccessMana, manamodels.ConsensusMana} {
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

// QueryAllowed returns if the mana plugin answers queries or not.
func (t *Tracker) QueryAllowed() (allowed bool) {
	// if debugging enabled, reply to the query
	// if debugging is not allowed, only reply when in sync
	// return deps.Tangle.Bootstrapped() || debuggingEnabled\

	// query allowed only when base mana vectors have been initialized
	return t.consensusManaVector != nil && t.accessManaVector != nil
}

func (t *Tracker) vectorByType(manaType manamodels.Type) (manaVector *manamodels.ManaBaseVector) {
	switch manaType {
	case manamodels.AccessMana:
		return t.accessManaVector
	case manamodels.ConsensusMana:
		return t.consensusManaVector
	default:
		panic("unknown mana type")
	}
}
