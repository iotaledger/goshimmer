package ledgerstate

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
)

// Snapshot defines a snapshot of the ledger state.
type Snapshot struct {
	Transactions     map[TransactionID]Record
	AccessManaByNode map[identity.ID]AccessMana
}

// AccessMana defines the info for the aMana snapshot.
type AccessMana struct {
	Value     float64
	Timestamp time.Time
}

// Record defines a record of the snapshot.
type Record struct {
	Essence        *TransactionEssence
	UnlockBlocks   UnlockBlocks
	UnspentOutputs []bool
}

// WriteTo writes the snapshot data to the given writer.
func (s *Snapshot) WriteTo(writer io.Writer) (int64, error) {
	var bytesWritten int64
	if err := binary.Write(writer, binary.LittleEndian, uint32(len(s.Transactions))); err != nil {
		return 0, fmt.Errorf("unable to write transactions count: %w", err)
	}
	bytesWritten += 4
	for transactionID, record := range s.Transactions {
		if err := binary.Write(writer, binary.LittleEndian, uint32(len(record.Essence.Bytes()))); err != nil {
			return 0, fmt.Errorf("unable to write length of transaction with %s: %w", transactionID, err)
		}
		bytesWritten += 4

		if err := binary.Write(writer, binary.LittleEndian, transactionID.Bytes()); err != nil {
			return 0, fmt.Errorf("unable to write transactionID with %s: %w", transactionID, err)
		}
		bytesWritten += TransactionIDLength

		if err := binary.Write(writer, binary.LittleEndian, record.Essence.Bytes()); err != nil {
			return 0, fmt.Errorf("unable to write transaction with %s: %w", transactionID, err)
		}
		bytesWritten += int64(len(record.Essence.Bytes()))

		// fmt.Printf("Writing unlockBlock : %s", s.Transactions)
		unlockBlocksLength := uint32(len(record.UnlockBlocks.Bytes()))
		if err := binary.Write(writer, binary.LittleEndian, unlockBlocksLength); err != nil {
			return 0, fmt.Errorf("unable to write unspent output index length with %s: %w", transactionID, err)
		}
		bytesWritten += 4

		if err := binary.Write(writer, binary.LittleEndian, record.UnlockBlocks.Bytes()); err != nil {
			return 0, fmt.Errorf("unable to write unlockBlocks with %s: %w", transactionID, err)
		}
		bytesWritten += int64(unlockBlocksLength)

		if err := binary.Write(writer, binary.LittleEndian, uint32(len(record.UnspentOutputs))); err != nil {
			return 0, fmt.Errorf("unable to write unspent output index length with %s: %w", transactionID, err)
		}
		bytesWritten += 4

		for _, unspentOutput := range record.UnspentOutputs {
			if err := binary.Write(writer, binary.LittleEndian, unspentOutput); err != nil {
				return 0, fmt.Errorf("unable to write unspent output index with %s: %w", transactionID, err)
			}
		}
		bytesWritten += int64(len(record.UnspentOutputs))
	}

	if err := binary.Write(writer, binary.LittleEndian, uint32(len(s.AccessManaByNode))); err != nil {
		return 0, fmt.Errorf("unable to write AccessMana count: %w", err)
	}
	bytesWritten += 4
	for nodeID, accessMana := range s.AccessManaByNode {
		if err := binary.Write(writer, binary.LittleEndian, nodeID.Bytes()); err != nil {
			return 0, fmt.Errorf("unable to write nodeID with %s: %w", nodeID, err)
		}
		bytesWritten += identity.IDLength
		if err := binary.Write(writer, binary.LittleEndian, accessMana.Value); err != nil {
			return 0, fmt.Errorf("unable to write access mana : %w", err)
		}
		bytesWritten += 8
		if err := binary.Write(writer, binary.LittleEndian, accessMana.Timestamp.Unix()); err != nil {
			return 0, fmt.Errorf("unable to write timestamp : %w", err)
		}
		bytesWritten += 8
	}

	return bytesWritten, nil
}

// ReadFrom reads the snapshot bytes from the given reader.
// This function overrides existing content of the snapshot.
func (s *Snapshot) ReadFrom(reader io.Reader) (int64, error) {
	bytesTransactions, err := s.readTransactions(reader)
	if err != nil {
		return bytesTransactions, err
	}

	bytesAccessMana, err := s.readAccessMana(reader)
	if err != nil {
		return bytesAccessMana, err
	}

	return bytesTransactions + bytesAccessMana, nil
}

// readTransactions reads the transactions from the snapshot.
func (s *Snapshot) readTransactions(reader io.Reader) (int64, error) {
	s.Transactions = make(map[TransactionID]Record)
	var bytesRead int64
	var transactionCount uint32

	// read Transactions
	if err := binary.Read(reader, binary.LittleEndian, &transactionCount); err != nil {
		return 0, fmt.Errorf("unable to read transaction count: %w", err)
	}
	bytesRead += 4

	for i := 0; i < int(transactionCount); i++ {
		var transactionLength uint32
		if err := binary.Read(reader, binary.LittleEndian, &transactionLength); err != nil {
			return 0, fmt.Errorf("unable to read length of transaction at index %d: %w", i, err)
		}
		bytesRead += 4

		transactionIDBytes := make([]byte, TransactionIDLength)
		if err := binary.Read(reader, binary.LittleEndian, &transactionIDBytes); err != nil {
			return 0, fmt.Errorf("unable to read transactionID: %w", err)
		}

		_, n, e := TransactionIDFromBytes(transactionIDBytes)
		if e != nil {
			return 0, fmt.Errorf("unable to parse transactionID at index %d: %w", i, e)
		}
		bytesRead += int64(n)

		transactionBytes := make([]byte, transactionLength)
		if err := binary.Read(reader, binary.LittleEndian, &transactionBytes); err != nil {
			return 0, fmt.Errorf("unable to read transaction at index %d: %w", i, err)
		}

		txEssence, n, err := TransactionEssenceFromBytes(transactionBytes)
		if err != nil {
			return 0, fmt.Errorf("unable to parse transaction at index %d: %w", i, err)
		}
		bytesRead += int64(n)

		var unlockBlockLength uint32
		if err = binary.Read(reader, binary.LittleEndian, &unlockBlockLength); err != nil {
			return 0, fmt.Errorf("unable to read length of unlockBlocks at index %d: %w", i, err)
		}
		bytesRead += 4

		unlockBlockBytes := make([]byte, unlockBlockLength)
		if err = binary.Read(reader, binary.LittleEndian, &unlockBlockBytes); err != nil {
			return 0, fmt.Errorf("unable to read transactionID: %w", err)
		}
		unlockBlocks, n, err := UnlockBlocksFromBytes(unlockBlockBytes)
		if err != nil {
			return 0, fmt.Errorf("unable to parse unlockblocks at index %d: %w", i, err)
		}
		bytesRead += int64(n)

		var unspentOutputsLength uint32
		if err := binary.Read(reader, binary.LittleEndian, &unspentOutputsLength); err != nil {
			return 0, fmt.Errorf("unable to read unspent outputs length at index %d: %w", i, err)
		}
		bytesRead += 4

		unspentOutputs := make([]bool, unspentOutputsLength)
		for j := 0; j < int(unspentOutputsLength); j++ {
			if err := binary.Read(reader, binary.LittleEndian, &unspentOutputs[j]); err != nil {
				return 0, fmt.Errorf("unable to read unspent output at index %d: %w", j, err)
			}
		}

		bytesRead += int64(unspentOutputsLength)
		id := NewTransaction(txEssence, unlockBlocks).ID()
		s.Transactions[id] = Record{
			Essence:        txEssence,
			UnlockBlocks:   unlockBlocks,
			UnspentOutputs: unspentOutputs,
		}
	}

	return bytesRead, nil
}

// readAccessMana reads the access mana from the snapshot.
func (s *Snapshot) readAccessMana(reader io.Reader) (int64, error) {
	s.AccessManaByNode = make(map[identity.ID]AccessMana)
	var bytesRead int64
	var accessManaCount uint32

	// read access mana
	if err := binary.Read(reader, binary.LittleEndian, &accessManaCount); err != nil {
		return 0, fmt.Errorf("unable to read AccessMana count: %w", err)
	}
	bytesRead += 4
	for i := 0; i < int(accessManaCount); i++ {
		nodeIDBytes := make([]byte, identity.IDLength)
		if err := binary.Read(reader, binary.LittleEndian, &nodeIDBytes); err != nil {
			return 0, fmt.Errorf("unable to read nodeID: %w", err)
		}
		bytesRead += identity.IDLength
		marshalutilNodeID := marshalutil.New(nodeIDBytes)
		nodeID, err := identity.IDFromMarshalUtil(marshalutilNodeID)
		if err != nil {
			return 0, fmt.Errorf("unable to parse nodeID: %w", err)
		}

		var accessMana float64
		if err := binary.Read(reader, binary.LittleEndian, &accessMana); err != nil {
			return 0, fmt.Errorf("unable to read access mana: %w", err)
		}
		bytesRead += 8

		var timestampUnix int64
		if err := binary.Read(reader, binary.LittleEndian, &timestampUnix); err != nil {
			return 0, fmt.Errorf("unable to read timestamp: %w", err)
		}
		bytesRead += 8
		timestamp := time.Unix(timestampUnix, 0)

		s.AccessManaByNode[nodeID] = AccessMana{
			Value:     accessMana,
			Timestamp: timestamp,
		}
	}

	return bytesRead, nil
}
