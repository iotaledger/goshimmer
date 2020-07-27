package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// config type that defines the config structure
type configuration struct {
	WebAPI string
}

// internal variable that holds the config
var config = configuration{}

// load the config file
func loadConfig() {
	// open config file
	file, err := os.Open("config.json")
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}

		if err = ioutil.WriteFile("config.json", []byte("{\n  \"WebAPI\": \"http://127.0.0.1:8080\"\n}"), 0644); err != nil {
			panic(err)
		}
		if file, err = os.Open("config.json"); err != nil {
			panic(err)
		}
	}
	defer file.Close()

	// decode config file
	if err = json.NewDecoder(file).Decode(&config); err != nil {
		panic(err)
	}
}
