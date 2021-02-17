package ledgerstate

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Snapshot defines a snapshot of the ledger state.
type Snapshot map[TransactionID]map[Address]*ColoredBalances

// WriteTo writes the snapshot data to the given writer in the following format:
// 	transaction_count(int64)
//	-> transaction_count * transaction_id(32byte)
//		->address_count(int64)
//			->address_count * address(33byte)
//				->balance_count(int64)
//					->balance_count * value(int64)+color(32byte)
func (s Snapshot) WriteTo(writer io.Writer) (int64, error) {
	var bytesWritten int64
	transactionCount := len(s)
	if err := binary.Write(writer, binary.LittleEndian, int64(transactionCount)); err != nil {
		return 0, fmt.Errorf("unable to write transactions count: %w", err)
	}
	bytesWritten += 8
	for txID, addresses := range s {
		if err := binary.Write(writer, binary.LittleEndian, txID); err != nil {
			return bytesWritten, fmt.Errorf("unable to write transaction ID: %w", err)
		}
		bytesWritten += TransactionIDLength
		if err := binary.Write(writer, binary.LittleEndian, int64(len(addresses))); err != nil {
			return bytesWritten, fmt.Errorf("unable to write address count: %w", err)
		}
		bytesWritten += 8
		for addr, balances := range addresses {
			if err := binary.Write(writer, binary.LittleEndian, addr.Bytes()); err != nil {
				return bytesWritten, fmt.Errorf("unable to write address: %w", err)
			}
			bytesWritten += AddressLength
			if err := binary.Write(writer, binary.LittleEndian, int64(balances.Size())); err != nil {
				return bytesWritten, fmt.Errorf("unable to write balance count: %w", err)
			}
			bytesWritten += 8
			var err error
			balances.ForEach(func(color Color, balance uint64) bool {
				if err = binary.Write(writer, binary.LittleEndian, balance); err != nil {
					err = fmt.Errorf("unable to write balance value: %w", err)
					return false
				}
				bytesWritten += 8
				if err = binary.Write(writer, binary.LittleEndian, color); err != nil {
					err = fmt.Errorf("unable to write balance color: %w", err)
					return false
				}
				bytesWritten += ColorLength
				return true
			})
			if err != nil {
				return bytesWritten, err
			}
		}
	}

	return bytesWritten, nil
}

// ReadFrom reads the snapshot bytes from the given reader.
// This function overrides existing content of the snapshot.
func (s Snapshot) ReadFrom(reader io.Reader) (int64, error) {
	var bytesRead int64
	var transactionCount int64
	if err := binary.Read(reader, binary.LittleEndian, &transactionCount); err != nil {
		return 0, fmt.Errorf("unable to read transaction count: %w", err)
	}
	bytesRead += 8

	var i int64
	for ; i < transactionCount; i++ {
		txIDBytes := make([]byte, TransactionIDLength)
		if err := binary.Read(reader, binary.LittleEndian, txIDBytes); err != nil {
			return bytesRead, fmt.Errorf("unable to read transaction ID: %w", err)
		}
		bytesRead += TransactionIDLength
		var addrCount int64
		if err := binary.Read(reader, binary.LittleEndian, &addrCount); err != nil {
			return bytesRead, fmt.Errorf("unable to read address count: %w", err)
		}
		bytesRead += 8
		txAddrMap := make(map[Address]*ColoredBalances, addrCount)
		var j int64
		for ; j < addrCount; j++ {
			addrBytes := make([]byte, AddressLength)
			if err := binary.Read(reader, binary.LittleEndian, addrBytes); err != nil {
				return bytesRead, fmt.Errorf("unable to read address: %w", err)
			}
			bytesRead += AddressLength
			var balanceCount int64
			if err := binary.Read(reader, binary.LittleEndian, &balanceCount); err != nil {
				return bytesRead, fmt.Errorf("unable to read balance count: %w", err)
			}
			bytesRead += 8

			balances := make(map[Color]uint64, balanceCount)
			var k int64
			for ; k < balanceCount; k++ {
				var value uint64
				if err := binary.Read(reader, binary.LittleEndian, &value); err != nil {
					return bytesRead, fmt.Errorf("unable to read balance value: %w", err)
				}
				bytesRead += 8
				color := Color{}
				if err := binary.Read(reader, binary.LittleEndian, &color); err != nil {
					return bytesRead, fmt.Errorf("unable to read balance color: %w", err)
				}
				bytesRead += ColorLength
				balances[color] = value
			}
			coloredBalances := NewColoredBalances(balances)
			addr, _, err := AddressFromBytes(addrBytes)
			if err != nil {
				return bytesRead, fmt.Errorf("unable to unmarshal address: %w", err)
			}
			txAddrMap[addr] = coloredBalances
		}
		txID, _, err := TransactionIDFromBytes(txIDBytes)
		if err != nil {
			return bytesRead, fmt.Errorf("unable to unmarshal txIDbytes: %w", err)
		}
		s[txID] = txAddrMap
	}

	return bytesRead, nil
}
