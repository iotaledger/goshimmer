package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/mr-tron/base58"
)

func main() {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("base64:%s\n", base64.StdEncoding.EncodeToString(b))
	fmt.Printf("base58:%s\n", base58.Encode(b))
}
