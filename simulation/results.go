package main

import (
	"fmt"
	"os"
	"sort"
)

func createDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
}

func linksToString(links map[int64]int) (output [][]string) {
	keys := make([]int, len(links))
	i := 0
	total := 0
	for k, v := range links {
		keys[i] = int(k)
		i++
		total += v
	}
	sort.Ints(keys)
	for _, key := range keys {
		record := []string{
			fmt.Sprintf("%v", key),
			fmt.Sprintf("%v", float64(links[int64(key)])/float64(total)),
		}
		output = append(output, record)
	}
	return output
}

func convergenceToString(c []Convergence) (output [][]string) {
	for _, line := range c {
		record := []string{
			fmt.Sprintf("%v", line.timestamp.Seconds()),
			fmt.Sprintf("%v", line.counter),
		}
		output = append(output, record)
	}
	return output
}
