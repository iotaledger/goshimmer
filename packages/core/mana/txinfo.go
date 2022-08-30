package mana

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/iotaledger/hive.go/core/stringify"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
)

// TxInfo holds information related to the transaction which we are processing for mana calculation.
type TxInfo struct {
	// Timestamp is the timestamp of the transaction.
	TimeStamp time.Time
	// TransactionID is the ID of the transaction.
	TransactionID utxo.TransactionID
	// TotalBalance is the amount of funds being transferred via the transaction.
	TotalBalance float64
	// PledgeID is a map of mana types and the node to which this transaction pledges its mana type to.
	PledgeID map[Type]identity.ID
	// InputInfos is a slice of InputInfo that holds mana related info about each input within the transaction.
	InputInfos []InputInfo
}

func (t *TxInfo) sumInputs() float64 {
	t.TotalBalance = 0
	for _, input := range t.InputInfos {
		t.TotalBalance += input.Amount
	}
	return t.TotalBalance
}

// InputInfo holds mana related info about an input within a transaction.
type InputInfo struct {
	// Amount is the balance of the input.
	Amount float64
	// PledgeID is a map of mana types and the node to which the transaction that created the output pledges its mana type to.
	PledgeID map[Type]identity.ID
	// InputID is the input consumed.
	InputID utxo.OutputID
}

// SnapshotNode defines the record for the mana snapshot of one node.
type SnapshotNode struct {
	AccessMana       *AccessManaSnapshot
	SortedTxSnapshot SortedTxSnapshot
}

func (s *SnapshotNode) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	s.AccessMana = new(AccessManaSnapshot)
	if err = s.AccessMana.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to read AccessMana: %w", err)
	}
	s.SortedTxSnapshot = make(SortedTxSnapshot, 0)
	if err = s.SortedTxSnapshot.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to read SortedTxSnapshot: %w", err)
	}

	return nil
}

func (s *SnapshotNode) AccessManaUpdateTime() (maxTime time.Time) {
	return s.AccessMana.Timestamp
}

func (s *SnapshotNode) AdjustAccessManaUpdateTime(diff time.Duration) {
	s.AccessMana.Timestamp = s.AccessMana.Timestamp.Add(diff)
}

// Bytes returns a serialized version of the SnapshotNode.
func (s *SnapshotNode) Bytes() (serialized []byte) {
	return marshalutil.New().
		Write(s.AccessMana).
		Write(s.SortedTxSnapshot).
		Bytes()
}

// String returns a human-readable version of the SnapshotNode.
func (s *SnapshotNode) String() (humanReadable string) {
	return stringify.Struct("SnapshotNode",
		stringify.NewStructField("AccessMana", s.AccessMana),
		stringify.NewStructField("SortedTxSnapshot", s.SortedTxSnapshot),
	)
}

// AccessManaSnapshot defines the record for the aMana snapshot of one node.
type AccessManaSnapshot struct {
	Value     float64
	Timestamp time.Time
}

// FromMarshalUtil reads the AccessManaSnapshot from a MarshalUtil.
func (a *AccessManaSnapshot) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if a.Value, err = marshalUtil.ReadFloat64(); err != nil {
		return errors.Errorf("failed to read Value: %w", err)
	}
	if a.Timestamp, err = marshalUtil.ReadTime(); err != nil {
		return errors.Errorf("failed to read Timestamp: %w", err)
	}

	return nil
}

// Bytes returns a serialized version of the AccessManaSnapshot.
func (a *AccessManaSnapshot) Bytes() (serialized []byte) {
	return marshalutil.New().
		WriteFloat64(a.Value).
		WriteTime(a.Timestamp).
		Bytes()
}

// String returns a human-readable version of the AccessManaSnapshot.
func (a AccessManaSnapshot) String() (humanReadable string) {
	return stringify.Struct("AccessManaSnapshot",
		stringify.NewStructField("Value", a.Value),
		stringify.NewStructField("Timestamp", a.Timestamp),
	)
}

// TxSnapshot defines the record of one transaction.
type TxSnapshot struct {
	Value     float64
	TxID      utxo.TransactionID
	Timestamp time.Time
}

func (t *TxSnapshot) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if t.Value, err = marshalUtil.ReadFloat64(); err != nil {
		return errors.Errorf("failed to read Value: %w", err)
	}

	txBytes, err := marshalUtil.ReadBytes(t.TxID.Length())
	if err != nil {
		return errors.Errorf("failed to read TxID bytes: %w", err)
	}

	if _, err = t.TxID.Decode(txBytes); err != nil {
		return errors.Errorf("failed to read TxID: %w", err)
	}

	if t.Timestamp, err = marshalUtil.ReadTime(); err != nil {
		return errors.Errorf("failed to read Timestamp: %w", err)
	}

	return nil
}

func (t *TxSnapshot) Bytes() (serialized []byte) {
	return marshalutil.New().
		WriteFloat64(t.Value).
		Write(t.TxID).
		WriteTime(t.Timestamp).
		Bytes()
}

// String returns a human-readable version of the TxSnapshot.
func (t *TxSnapshot) String() (humanReadable string) {
	return stringify.Struct("TxSnapshot",
		stringify.NewStructField("Value", t.Value),
		stringify.NewStructField("TxID", t.TxID),
		stringify.NewStructField("Timestamp", t.Timestamp),
	)
}

// SortedTxSnapshot defines a list of SnapshotInfo sorted by timestamp.
type SortedTxSnapshot []*TxSnapshot

func (s *SortedTxSnapshot) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	snapshotCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return errors.Errorf("failed to read snapshot count: %w", err)
	}

	for i := uint64(0); i < snapshotCount; i++ {
		txSnapshot := new(TxSnapshot)
		if err = txSnapshot.FromMarshalUtil(marshalUtil); err != nil {
			return errors.Errorf("failed to read snapshot %d: %w", i, err)
		}

		*s = append(*s, txSnapshot)
	}

	return nil
}

func (s SortedTxSnapshot) Len() int           { return len(s) }
func (s SortedTxSnapshot) Less(i, j int) bool { return s[i].Timestamp.Before(s[j].Timestamp) }
func (s SortedTxSnapshot) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s SortedTxSnapshot) Bytes() (serialized []byte) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteUint64(uint64(len(s)))
	for _, txSnapshot := range s {
		marshalUtil.Write(txSnapshot)
	}

	return marshalUtil.Bytes()
}

func (s SortedTxSnapshot) String() (humanReadable string) {
	humanReadable += "["

	for i := 0; i < len(s); i++ {
		humanReadable += s[i].String() + ", "
	}

	if len(humanReadable) >= 2 {
		humanReadable = humanReadable[:len(humanReadable)-2]
	}

	humanReadable += "]"

	return humanReadable
}
