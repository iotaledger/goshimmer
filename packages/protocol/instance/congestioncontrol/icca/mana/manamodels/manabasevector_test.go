package manamodels

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"
)

var (
	baseTime = time.Now()
)

func randIssuerID() identity.ID {
	return identity.GenerateIdentity().ID()
}

func TestNewBaseManaVector_Consensus(t *testing.T) {
	bmvCons := NewManaBaseVector(ConsensusMana)
	assert.Equal(t, ConsensusMana, bmvCons.Type())
	assert.Equal(t, map[identity.ID]*ManaBase{}, bmvCons.M.Vector)
}

func TestConsensusBaseManaVector_Type(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)
	vectorType := bmv.Type()
	assert.Equal(t, ConsensusMana, vectorType)
}

func TestConsensusBaseManaVector_Size(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)
	assert.Equal(t, 0, bmv.Size())

	for i := 0; i < 10; i++ {
		bmv.SetMana(randIssuerID(), NewManaBase(float64(i)))
	}
	assert.Equal(t, 10, bmv.Size())
}

func TestConsensusBaseManaVector_Has(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)
	randID := randIssuerID()

	has := bmv.Has(randID)
	assert.False(t, has)

	bmv.SetMana(randID, NewManaBase(0))
	has = bmv.Has(randID)
	assert.True(t, has)
}

func TestConsensusBaseManaVector_GetMana(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)
	randID := randIssuerID()
	manaValue, _, err := bmv.GetMana(randID)
	assert.Equal(t, 0.0, manaValue)
	assert.Error(t, err)

	bmv.SetMana(randID, NewManaBase(0))
	manaValue, _, err = bmv.GetMana(randID)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, manaValue)

	bmv.SetMana(randID, NewManaBase(10.0))

	manaValue, _, err = bmv.GetMana(randID)
	assert.NoError(t, err)
	assert.Equal(t, 10.0, manaValue)
}

func TestConsensusBaseManaVector_ForEach(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)

	for i := 0; i < 10000; i++ {
		bmv.SetMana(randIssuerID(), NewManaBase(1.0))
	}

	// fore each should iterate over all elements
	sum := 0.0
	bmv.ForEach(func(ID identity.ID, bm BaseMana) bool {
		sum += bm.BaseValue()
		return true
	})
	assert.Equal(t, 10000.0, sum)

	// for each should stop if false is returned from callback
	sum = 0.0
	bmv.ForEach(func(ID identity.ID, bm BaseMana) bool {
		if sum >= 5000.0 {
			return false
		}
		sum += bm.BaseValue()
		return true
	})

	assert.Equal(t, 5000.0, sum)
}

func TestConsensusBaseManaVector_GetManaMap(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)

	// empty vector returns empty map
	manaMap, _, err := bmv.GetManaMap()
	assert.NoError(t, err)
	assert.Empty(t, manaMap)

	issuerIDs := map[identity.ID]int{}

	for i := 0; i < 100; i++ {
		id := randIssuerID()
		bmv.SetMana(id, NewManaBase(10.0))
		issuerIDs[id] = 0
	}

	manaMap, _, err = bmv.GetManaMap()
	assert.NoError(t, err)
	assert.Equal(t, 100, len(manaMap))
	for issuerID, manaValue := range manaMap {
		assert.Equal(t, 10.0, manaValue)
		assert.Contains(t, issuerIDs, issuerID)
		delete(issuerIDs, issuerID)
	}
	assert.Empty(t, issuerIDs)
}

func TestConsensusBaseManaVector_GetHighestManaIssuers(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)

	issuerIDs := make([]identity.ID, 10)

	for i := 0; i < 10; i++ {
		issuerIDs[i] = randIssuerID()
		bmv.SetMana(issuerIDs[i], NewManaBase(float64(i)))
	}

	// requesting the top mana holder
	result, _, err := bmv.GetHighestManaIssuers(1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, issuerIDs[9], result[0].ID)
	assert.Equal(t, 9.0, result[0].Mana)

	// requesting top 3 mana holders
	result, _, err = bmv.GetHighestManaIssuers(3)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result))
	assert.Equal(t, 9.0, result[0].Mana)
	for index, value := range result {
		if index < 2 {
			// it's greater than the next one
			assert.True(t, value.Mana > result[index+1].Mana)
		}
		assert.Equal(t, issuerIDs[9-index], value.ID)
	}

	// requesting more, than there currently are in the vector
	result, _, err = bmv.GetHighestManaIssuers(20)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(result))
	for index, value := range result {
		assert.Equal(t, issuerIDs[9-index], value.ID)
	}
}

func TestConsensusBaseManaVector_GetHighestManaIssuersFraction(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)

	issuerIDs := make([]identity.ID, 10)

	baseTime = time.Now()

	for i := 0; i < 10; i++ {
		issuerIDs[i] = randIssuerID()
		bmv.SetMana(issuerIDs[i], NewManaBase(float64(i)))
	}

	// requesting minus value
	result, _, err := bmv.GetHighestManaIssuersFraction(-0.1)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(result))
	assert.Equal(t, issuerIDs[9], result[0].ID)
	assert.Equal(t, 9.0, result[0].Mana)

	// requesting the holders of top 10% of mana
	result, _, err = bmv.GetHighestManaIssuersFraction(0.1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, issuerIDs[9], result[0].ID)
	assert.Equal(t, 9.0, result[0].Mana)

	// requesting holders of top 50% of mana
	result, _, err = bmv.GetHighestManaIssuersFraction(0.5)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result))
	assert.Equal(t, 9.0, result[0].Mana)
	for index, value := range result {
		if index < 2 {
			// it's greater than the next one
			assert.True(t, value.Mana > result[index+1].Mana)
		}
		assert.Equal(t, issuerIDs[9-index], value.ID)
	}

	// requesting more, than there currently are in the vector
	result, _, err = bmv.GetHighestManaIssuersFraction(1.1)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(result))
	for index, value := range result {
		assert.Equal(t, issuerIDs[9-index], value.ID)
	}
}

func TestConsensusBaseManaVector_SetMana(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)
	issuerIDs := make([]identity.ID, 10)
	for i := 0; i < 10; i++ {
		issuerIDs[i] = randIssuerID()
		bmv.SetMana(issuerIDs[i], NewManaBase(float64(i)))
	}
	for i := 0; i < 10; i++ {
		assert.Equal(t, NewManaBase(float64(i)), bmv.M.Vector[issuerIDs[i]])
	}
}

func TestConsensusBaseManaVector_ToPersistables(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)
	id1 := randIssuerID()
	id2 := randIssuerID()
	data := map[identity.ID]float64{
		id1: 1,
		id2: 10,
	}
	bmv.SetMana(id1, NewManaBase(data[id1]))
	bmv.SetMana(id2, NewManaBase(data[id2]))

	persistables := bmv.ToPersistables()

	assert.Equal(t, 2, len(persistables))
	for _, p := range persistables {
		assert.Equal(t, p.ManaType(), ConsensusMana)
		assert.Equal(t, 1, len(p.BaseValues()))
		assert.Equal(t, data[p.IssuerID()], p.BaseValues()[0])
		delete(data, p.IssuerID())
	}
	assert.Equal(t, 0, len(data))
}

func TestConsensusBaseManaVector_FromPersistable(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		id := randIssuerID()
		p := NewPersistableBaseMana(id, ConsensusMana, []float64{10}, []float64{100}, baseTime)
		bmv := NewManaBaseVector(ConsensusMana)
		assert.False(t, bmv.Has(id))
		err := bmv.FromPersistable(p)
		assert.NoError(t, err)
		assert.True(t, bmv.Has(id))
		assert.Equal(t, 1, bmv.Size())
		bmValue := bmv.M.Vector[id]
		assert.Equal(t, 10.0, bmValue.BaseValue())
	})

	t.Run("CASE: Wrong number of base values", func(t *testing.T) {
		p := NewPersistableBaseMana(randIssuerID(), ConsensusMana, []float64{0, 0}, []float64{0}, baseTime)

		bmv := NewManaBaseVector(ConsensusMana)

		err := bmv.FromPersistable(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has 2 base values instead of 1")
	})
}

func TestConsensusBaseManaVector_ToAndFromPersistable(t *testing.T) {
	bmv := NewManaBaseVector(ConsensusMana)
	id1 := randIssuerID()
	id2 := randIssuerID()
	data := map[identity.ID]float64{
		id1: 1,
		id2: 10,
	}
	bmv.SetMana(id1, NewManaBase(data[id1]))
	bmv.SetMana(id2, NewManaBase(data[id2]))

	persistables := bmv.ToPersistables()

	var restoredBmv *ManaBaseVector
	restoredBmv = NewManaBaseVector(ConsensusMana)

	for _, p := range persistables {
		err := restoredBmv.FromPersistable(p)
		assert.NoError(t, err)
	}
	assert.Equal(t, bmv.M.Vector, restoredBmv.M.Vector)
}
