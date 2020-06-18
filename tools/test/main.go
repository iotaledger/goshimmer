package main

import (
	"fmt"

	"github.com/mr-tron/base58"
)

func main() {
	x, err := base58.Decode("7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih")
	_ = err
	fmt.Println(x)
}
