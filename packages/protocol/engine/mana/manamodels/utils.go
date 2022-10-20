package manamodels

import (
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

// IssuerMapStr is a IssuerMap but with string id.
type IssuerMapStr map[string]int64

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

// GetPercentile returns the top percentile the issuer belongs to relative to the network in terms of mana.
func (n IssuerMap) GetPercentile(issuer identity.ID) (float64, error) {
	if len(n) == 0 {
		return 0, nil
	}
	value, ok := n[issuer]
	if !ok {
		return 0, ErrIssuerNotFoundInBaseManaVector
	}
	nBelow := 0.0
	for _, val := range n {
		if val < value {
			nBelow++
		}
	}

	return (nBelow / float64(len(n))) * 100, nil
}
