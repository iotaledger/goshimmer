package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
)

func initCSV(records [][]string, filename string) error {
	createDirIfNotExist("data")
	f, err := os.Create("data/result_" + filename + ".csv")
	if err != nil {
		fmt.Printf("error creating file: %v", err)
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err = w.WriteAll(records); err != nil {
		log.Fatalln("error writing csv:", err)
	}

	return err
}

func writeCSV(records [][]string, filename string, header ...[]string) error {
	var err error
	if header != nil {
		err = initCSV(header, filename) // requires format into [][]string
	} else {
		err = initCSV([][]string{}, filename) // requires format into [][]string
	}
	if err != nil {
		log.Fatalln("error initializing csv:", err)
	}
	f, err := os.OpenFile("data/result_"+filename+".csv", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Printf("error creating file: %v", err)
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	err = w.WriteAll(records) // calls Flush internally
	if err != nil {
		log.Fatalln("error writing csv:", err)
	}
	return err
}
