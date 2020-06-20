package tangle

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// Snapshot defines a snapshot of the ledger state.
type Snapshot map[transaction.ID]map[address.Address][]*balance.Balance

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
		bytesWritten += transaction.IDLength
		if err := binary.Write(writer, binary.LittleEndian, int64(len(addresses))); err != nil {
			return bytesWritten, fmt.Errorf("unable to write address count: %w", err)
		}
		bytesWritten += 8
		for addr, balances := range addresses {
			if err := binary.Write(writer, binary.LittleEndian, addr); err != nil {
				return bytesWritten, fmt.Errorf("unable to write address: %w", err)
			}
			bytesWritten += address.Length
			if err := binary.Write(writer, binary.LittleEndian, int64(len(balances))); err != nil {
				return bytesWritten, fmt.Errorf("unable to write balance count: %w", err)
			}
			bytesWritten += 8
			for _, bal := range balances {
				if err := binary.Write(writer, binary.LittleEndian, bal.Value); err != nil {
					return bytesWritten, fmt.Errorf("unable to write balance value: %w", err)
				}
				bytesWritten += 8
				if err := binary.Write(writer, binary.LittleEndian, bal.Color); err != nil {
					return bytesWritten, fmt.Errorf("unable to write balance color: %w", err)
				}
				bytesWritten += balance.ColorLength
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
		txIDBytes := make([]byte, transaction.IDLength)
		if err := binary.Read(reader, binary.LittleEndian, txIDBytes); err != nil {
			return bytesRead, fmt.Errorf("unable to read transaction ID: %w", err)
		}
		bytesRead += transaction.IDLength
		var addrCount int64
		if err := binary.Read(reader, binary.LittleEndian, &addrCount); err != nil {
			return bytesRead, fmt.Errorf("unable to read address count: %w", err)
		}
		bytesRead += 8
		txAddrMap := make(map[address.Address][]*balance.Balance, addrCount)
		var j int64
		for ; j < addrCount; j++ {
			addrBytes := make([]byte, address.Length)
			if err := binary.Read(reader, binary.LittleEndian, addrBytes); err != nil {
				return bytesRead, fmt.Errorf("unable to read address: %w", err)
			}
			bytesRead += address.Length
			var balanceCount int64
			if err := binary.Read(reader, binary.LittleEndian, &balanceCount); err != nil {
				return bytesRead, fmt.Errorf("unable to read balance count: %w", err)
			}
			bytesRead += 8

			balances := make([]*balance.Balance, balanceCount)
			var k int64
			for ; k < balanceCount; k++ {
				var value int64
				if err := binary.Read(reader, binary.LittleEndian, &value); err != nil {
					return bytesRead, fmt.Errorf("unable to read balance value: %w", err)
				}
				bytesRead += 8
				color := balance.Color{}
				if err := binary.Read(reader, binary.LittleEndian, &color); err != nil {
					return bytesRead, fmt.Errorf("unable to read balance color: %w", err)
				}
				bytesRead += balance.ColorLength
				balances[k] = &balance.Balance{Value: value, Color: color}
			}
			addr := address.Address{}
			copy(addr[:], addrBytes)
			txAddrMap[addr] = balances
		}
		txID := transaction.ID{}
		copy(txID[:], txIDBytes)
		s[txID] = txAddrMap
	}

	return bytesRead, nil
}
