package id

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	var id Id

	idBytes := make([]byte, Length)
	idBytes[0] = 1

	restoredId, err, _ := FromBytes(idBytes, &id)
	if err != nil {
		panic(err)
	}

	fmt.Println(id)
	fmt.Println(restoredId)
}
