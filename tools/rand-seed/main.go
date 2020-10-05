package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/mr-tron/base58"
)

func main() {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println(err)
		return
	}

	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile("random-seed.txt", os.O_WRONLY|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	_ = f.Truncate(0)

	f.WriteString("base64:" + base64.StdEncoding.EncodeToString(b) + "\n")
	f.WriteString("base58:" + base58.Encode(b))

	fmt.Println("New random seed generated (both base64 and base58 encoded) and written in random-seed.txt")
}
