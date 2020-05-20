package address

import (
	"bytes"
	"sort"
)

type sortedAddresses []Address

func (s sortedAddresses) Len() int {
	return len(s)
}

func (s sortedAddresses) Less(i, j int) bool {
	return bytes.Compare(s[i][:], s[j][:]) < 0
}

func (s sortedAddresses) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Sort function sorts the slice of addresses
func Sort(addresses []Address) {
	sort.Sort(sortedAddresses(addresses))
}
