package wallet

import (
	"runtime"

	"github.com/iotaledger/hive.go/bitmask"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
)

// AddressManager is an manager struct that allows us to keep track of the used and spent addresses.
type AddressManager struct {
	// state of the wallet
	seed             *seed.Seed
	lastAddressIndex uint64
	spentAddresses   []bitmask.BitMask

	// internal variables for faster access
	firstUnspentAddressIndex uint64
	lastUnspentAddressIndex  uint64
}

// NewAddressManager is the constructor for the AddressManager type.
func NewAddressManager(seed *seed.Seed, lastAddressIndex uint64, spentAddresses []bitmask.BitMask) (addressManager *AddressManager) {
	defer runtime.KeepAlive(spentAddresses)

	addressManager = &AddressManager{
		seed:             seed,
		lastAddressIndex: lastAddressIndex,
		spentAddresses:   spentAddresses,
	}
	addressManager.updateFirstUnspentAddressIndex()
	addressManager.updateLastUnspentAddressIndex()

	return
}

// Address returns the address that belongs to the given index.
func (addressManager *AddressManager) Address(addressIndex uint64) address.Address {
	// update lastUnspentAddressIndex if necessary
	addressManager.spentAddressIndexes(addressIndex)

	return addressManager.seed.Address(addressIndex)
}

// Addresses returns a list of all addresses of the wallet.
func (addressManager *AddressManager) Addresses() (addresses []address.Address) {
	addresses = make([]address.Address, addressManager.lastAddressIndex+1)
	for i := uint64(0); i <= addressManager.lastAddressIndex; i++ {
		addresses[i] = addressManager.Address(i)
	}

	return
}

// UnspentAddresses returns a list of all unspent addresses of the wallet.
func (addressManager *AddressManager) UnspentAddresses() (addresses []address.Address) {
	addresses = make([]address.Address, 0)
	for i := addressManager.firstUnspentAddressIndex; i <= addressManager.lastAddressIndex; i++ {
		if !addressManager.IsAddressSpent(i) {
			addresses = append(addresses, addressManager.Address(i))
		}
	}

	return
}

// SpentAddresses returns a list of all spent addresses of the wallet.
func (addressManager *AddressManager) SpentAddresses() (addresses []address.Address) {
	addresses = make([]address.Address, 0)
	for i := uint64(0); i <= addressManager.lastAddressIndex; i++ {
		if addressManager.IsAddressSpent(i) {
			addresses = append(addresses, addressManager.Address(i))
		}
	}

	return
}

// FirstUnspentAddress returns the first unspent address that we know.
func (addressManager *AddressManager) FirstUnspentAddress() address.Address {
	return addressManager.Address(addressManager.firstUnspentAddressIndex)
}

// LastUnspentAddress returns the last unspent address that we know.
func (addressManager *AddressManager) LastUnspentAddress() address.Address {
	return addressManager.Address(addressManager.lastUnspentAddressIndex)
}

// NewAddress generates and returns a new unused address.
func (addressManager *AddressManager) NewAddress() address.Address {
	return addressManager.Address(addressManager.lastAddressIndex + 1)
}

// MarkAddressSpent marks the given address as spent.
func (addressManager *AddressManager) MarkAddressSpent(addressIndex uint64) {
	// determine indexes
	sliceIndex, bitIndex := addressManager.spentAddressIndexes(addressIndex)

	// mark address as spent
	addressManager.spentAddresses[sliceIndex] = addressManager.spentAddresses[sliceIndex].SetBit(uint(bitIndex))

	// update spent address indexes
	if addressIndex == addressManager.firstUnspentAddressIndex {
		addressManager.updateFirstUnspentAddressIndex()
	}
	if addressIndex == addressManager.lastUnspentAddressIndex {
		addressManager.updateLastUnspentAddressIndex()
	}
}

// IsAddressSpent returns true if the address given by the address index was spent already.
func (addressManager *AddressManager) IsAddressSpent(addressIndex uint64) bool {
	sliceIndex, bitIndex := addressManager.spentAddressIndexes(addressIndex)

	return addressManager.spentAddresses[sliceIndex].HasBit(uint(bitIndex))
}

// spentAddressIndexes retrieves the indexes for the internal representation of the spend addresses bitmask slice that
// belongs to the given address index. It automatically increases the capacity and updates the lastAddressIndex and the
// lastUnspentAddressIndex if a new address is generated for the first time.
func (addressManager *AddressManager) spentAddressIndexes(addressIndex uint64) (sliceIndex uint64, bitIndex uint64) {
	// calculate result
	spentAddressesCapacity := uint64(len(addressManager.spentAddresses))
	sliceIndex = addressIndex / 8
	bitIndex = addressIndex % 8

	// extend capacity to make space for the requested index
	if sliceIndex+1 > spentAddressesCapacity {
		addressManager.spentAddresses = append(addressManager.spentAddresses, make([]bitmask.BitMask, sliceIndex-spentAddressesCapacity+1)...)
	}

	// update lastAddressIndex if the index is bigger
	if addressIndex > addressManager.lastAddressIndex {
		addressManager.lastAddressIndex = addressIndex
	}

	// update lastUnspentAddressIndex if necessary
	if addressIndex > addressManager.lastUnspentAddressIndex && !addressManager.spentAddresses[sliceIndex].HasBit(uint(bitIndex)) {
		addressManager.lastUnspentAddressIndex = addressIndex
	}

	return
}

// updateFirstUnspentAddressIndex searches for the first unspent address and updates the firstUnspentAddressIndex.
func (addressManager *AddressManager) updateFirstUnspentAddressIndex() {
	for i := addressManager.firstUnspentAddressIndex; true; i++ {
		if !addressManager.IsAddressSpent(i) {
			addressManager.firstUnspentAddressIndex = i

			return
		}
	}
}

// updateLastUnspentAddressIndex searches for the last unspent address and updates the lastUnspentAddressIndex.
func (addressManager *AddressManager) updateLastUnspentAddressIndex() {
	// search for last unspent address
	for i := addressManager.lastUnspentAddressIndex; true; i-- {
		if !addressManager.IsAddressSpent(i) {
			addressManager.lastUnspentAddressIndex = i

			return
		}

		if i == 0 {
			break
		}
	}

	// or generate a new unspent address
	addressManager.NewAddress()
}
