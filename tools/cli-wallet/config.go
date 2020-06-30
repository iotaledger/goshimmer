package main

import (
	"encoding/json"
	"os"
)

type Configuration struct {
	WebAPI string
}

// internal variable that holds the config
var config = Configuration{}

// parse the config upon launching the cli-wallet
func init() {
	// open config file
	file, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// decode config file
	if err = json.NewDecoder(file).Decode(&config); err != nil {
		panic(err)
	}
}
