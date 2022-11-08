package manamodels

import (
	"sort"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/mr-tron/base58"
)

// Issuer represents a issuer and its mana value.
type Issuer struct {
	ID   identity.ID
	Mana int64
}

// IssuerStr defines a issuer and its mana value.
// The issuer ID is stringified.
type IssuerStr struct {
	// TODO: rename JSON fields here and in dashboard/client library
	ShortIssuerID string `json:"shortNodeID"`
	IssuerID      string `json:"nodeID"`
	Mana          int64  `json:"mana"`
}

// ToIssuerStr converts a Issuer to a Nodestr
func (n Issuer) ToIssuerStr() IssuerStr {
	return IssuerStr{
		ShortIssuerID: n.ID.String(),
		IssuerID:      base58.Encode(lo.PanicOnErr(n.ID.Bytes())),
		Mana:          n.Mana,
	}
}

// IssuerMap is a map of issuerID and mana value.
type IssuerMap map[identity.ID]int64

// ToIssuerStrList converts a IssuerMap to list of IssuerStr.
func (n IssuerMap) ToIssuerStrList() []IssuerStr {
	var list []IssuerStr
	for ID, val := range n {
		list = append(list, IssuerStr{
			ShortIssuerID: ID.String(),
			IssuerID:      base58.Encode(lo.PanicOnErr(ID.Bytes())),
			Mana:          val,
		})
	}
	return list
}

// Percentile returns the top percentile the issuer belongs to relative to the network in terms of mana.
func Percentile(id identity.ID, m map[identity.ID]int64) (percentileValue float64) {
	if len(m) == 0 {
		return 0
	}
	value, ok := m[id]
	if !ok {
		return 0
	}
	nBelow := 0.0
	for _, val := range m {
		if val < value {
			nBelow++
		}
	}

	return (nBelow / float64(len(m))) * 100
}

// GetHighestManaIssuers return the n-highest mana issuers in descending order.
// It also updates the mana values for each issuer.
// If n is zero, it returns all issuers.
func GetHighestManaIssuers(n uint, m map[identity.ID]int64) (res []Issuer, t time.Time, err error) {
	t = time.Now()
	err = func() error {
		// don't lock the vector after this func returns
		for id, mana := range m {
			res = append(res, Issuer{
				ID:   id,
				Mana: mana,
			})
		}
		return nil
	}()
	if err != nil {
		return nil, t, err
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Mana > res[j].Mana
	})

	if n == 0 || int(n) >= len(res) {
		return
	}
	res = res[:n]
	return
}

