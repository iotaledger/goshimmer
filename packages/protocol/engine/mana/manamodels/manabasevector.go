package manamodels

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/storage/models"
)

// ManaBaseVector represents a base mana vector.
type ManaBaseVector struct {
	model.Mutable[ManaBaseVector, *ManaBaseVector, manaBaseVectorModel] `serix:"0"`
}

// NewManaBaseVector creates and returns a new base mana vector for the specified type.
func NewManaBaseVector(manaType Type) *ManaBaseVector {
	return model.NewMutable[ManaBaseVector](&manaBaseVectorModel{
		Type:   manaType,
		Vector: make(map[identity.ID]*ManaBase),
	})
}

type manaBaseVectorModel struct {
	Type   Type                      `serix:"0"`
	Vector map[identity.ID]*ManaBase `serix:"1"`
}

// Vector returns the ManaBase vector.
func (m *ManaBaseVector) Vector() map[identity.ID]*ManaBase {
	m.RLock()
	defer m.RUnlock()
	return m.M.Vector
}

// Type returns the type of this mana vector.
func (m *ManaBaseVector) Type() Type {
	return m.M.Type
}

// Size returns the size of this mana vector.
func (m *ManaBaseVector) Size() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.M.Vector)
}

// Has returns if the given issuer has mana defined in the vector.
func (m *ManaBaseVector) Has(issuerID identity.ID) bool {
	m.RLock()
	defer m.RUnlock()
	_, exists := m.M.Vector[issuerID]
	return exists
}

// InitializeWithData initializes the mana vector data.
func (m *ManaBaseVector) InitializeWithData(dataByIssuer map[identity.ID]int64) {
	m.Lock()
	defer m.Unlock()
	for issuerID, value := range dataByIssuer {
		m.M.Vector[issuerID] = NewManaBase(value)
	}
}

func (m *ManaBaseVector) GetIDBasedOnManaType(output *models.OutputWithMetadata) (pledgeID identity.ID) {
	if m.Type() == ConsensusMana {
		return output.ConsensusManaPledgeID()
	}
	return output.AccessManaPledgeID()
}

func (m *ManaBaseVector) GetOldManaAndRevoke(oldPledgeIssuerID identity.ID, amount int64) (oldMana ManaBase) {
	if _, exist := m.M.Vector[oldPledgeIssuerID]; !exist {
		// first time we see this issuer
		m.M.Vector[oldPledgeIssuerID] = NewManaBase(0)
	}
	// save old mana
	oldMana = *m.M.Vector[oldPledgeIssuerID]
	// revoke BM1
	err := m.M.Vector[oldPledgeIssuerID].revoke(amount)
	if errors.Is(err, ErrBaseManaNegative) {
		panic(fmt.Sprintf("Revoking %d base mana 1 from issuer %s results in negative balance", amount, oldPledgeIssuerID.String()))
	}
	return
}

func (m *ManaBaseVector) GetOldManaAndPledge(newPledgeIssuerID identity.ID, totalBalance int64) (oldMana ManaBase) {
	if _, exist := m.M.Vector[newPledgeIssuerID]; !exist {
		// first time we see this issuer
		m.M.Vector[newPledgeIssuerID] = NewManaBase(0)
	}
	// save it for proper event trigger
	oldMana = *m.M.Vector[newPledgeIssuerID]
	// actually pledge and update
	m.M.Vector[newPledgeIssuerID].pledge(totalBalance)
	return
}

// GetMana returns the Effective Base Mana.
func (m *ManaBaseVector) GetMana(issuerID identity.ID) (manaAmount int64, t time.Time, err error) {
	m.Lock()
	defer m.Unlock()
	manaAmount, err = m.getMana(issuerID)
	t = time.Now()
	return manaAmount, t, err
}

// GetManaMap returns mana perception of the issuer.
func (m *ManaBaseVector) GetManaMap() (res IssuerMap, t time.Time, err error) {
	m.Lock()
	defer m.Unlock()
	t = time.Now()
	res = make(map[identity.ID]int64, len(m.M.Vector))
	for ID, val := range m.M.Vector {
		res[ID] = val.BaseValue()
	}
	return
}

// GetHighestManaIssuers return the n-highest mana issuers in descending order.
// It also updates the mana values for each issuer.
// If n is zero, it returns all issuers.
func (m *ManaBaseVector) GetHighestManaIssuers(n uint) (res []Issuer, t time.Time, err error) {
	t = time.Now()
	err = func() error {
		// don't lock the vector after this func returns
		m.Lock()
		defer m.Unlock()
		for ID := range m.M.Vector {
			var mana int64
			mana, err = m.getMana(ID)
			if err != nil {
				return err
			}
			res = append(res, Issuer{
				ID:   ID,
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

// GetHighestManaIssuersFraction returns the highest mana that own 'p' percent of total mana.
// It also updates the mana values for each issuer.
// If p is zero or greater than one, it returns all issuers.
func (m *ManaBaseVector) GetHighestManaIssuersFraction(p float64) (res []Issuer, t time.Time, err error) {
	emptyIssuerID := identity.ID{}
	totalMana := int64(0)
	t = time.Now()
	err = func() error {
		// don't lock the vector after this func returns
		m.Lock()
		defer m.Unlock()
		for ID := range m.M.Vector {
			// skip the empty issuer ID
			if bytes.Equal(ID[:], emptyIssuerID[:]) {
				continue
			}

			var mana int64
			mana, err = m.getMana(ID)
			if err != nil {
				return err
			}
			res = append(res, Issuer{
				ID:   ID,
				Mana: mana,
			})
			totalMana += mana
		}
		return nil
	}()
	if err != nil {
		return nil, t, err
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Mana > res[j].Mana
	})

	// how much mana is p percent of total mana
	manaThreshold := int64(p * float64(totalMana))
	// include issuers as long as their counted mana is less than the threshold
	manaCounted := int64(0)
	var n uint
	for n = 0; int(n) < len(res) && manaCounted < manaThreshold; n++ {
		manaCounted += res[n].Mana
	}

	if n == 0 || int(n) >= len(res) {
		return
	}
	res = res[:n]
	return res, t, err
}

// SetMana sets the base mana for a issuer.
func (m *ManaBaseVector) SetMana(issuerID identity.ID, bm BaseMana) {
	m.Lock()
	defer m.Unlock()
	m.M.Vector[issuerID] = bm.(*ManaBase)
}

// ForEach iterates over the vector and calls the provided callback.
func (m *ManaBaseVector) ForEach(callback func(ID identity.ID, bm BaseMana) bool) {
	// lock to be on the safe side, although callback might just read
	m.Lock()
	defer m.Unlock()
	for issuerID, baseMana := range m.M.Vector {
		if !callback(issuerID, baseMana) {
			return
		}
	}
}

// ToPersistables converts the baseManaVector to a list of persistable mana objects.
func (m *ManaBaseVector) ToPersistables() []*PersistableBaseMana {
	m.RLock()
	defer m.RUnlock()
	var result []*PersistableBaseMana
	for issuerID, bm := range m.M.Vector {
		pbm := NewPersistableBaseMana(issuerID, m.Type(), []int64{bm.BaseValue()}, nil, time.Time{})
		result = append(result, pbm)
	}
	return result
}

// FromPersistable fills the BaseManaVector from persistable mana objects.
func (m *ManaBaseVector) FromPersistable(p *PersistableBaseMana) (err error) {
	if len(p.BaseValues()) != 1 {
		err = errors.Errorf("persistable mana object has %d base values instead of 1", len(p.BaseValues()))
		return
	}
	m.Lock()
	defer m.Unlock()
	m.M.Vector[p.IssuerID()] = model.NewMutable[ManaBase](&manaBaseModel{Value: p.BaseValues()[0]})
	return
}

// RemoveZeroIssuers removes the zero mana issuers from the vector.
func (m *ManaBaseVector) RemoveZeroIssuers() {
	m.Lock()
	defer m.Unlock()
	for issuerID, baseMana := range m.M.Vector {
		if baseMana.BaseValue() == 0 {
			delete(m.M.Vector, issuerID)
		}
	}
}

// // Region Internal methods ////

// getMana returns the consensus mana.
func (m *ManaBaseVector) getMana(issuerID identity.ID) (int64, error) {
	if _, exist := m.M.Vector[issuerID]; !exist {
		return 0.0, ErrIssuerNotFoundInBaseManaVector
	}

	baseMana := m.M.Vector[issuerID]
	return baseMana.BaseValue(), nil
}
