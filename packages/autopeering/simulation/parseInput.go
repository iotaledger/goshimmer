package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
)

func parseInput(filename string) *Param {

	P := &Param{}

	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if !strings.Contains(s.Text(), "#") && len(s.Text()) > 0 {
			param := strings.Split(s.Text(), "=")
			val := strings.Split(param[1], ",")
			//store the current set of parameter sets

			err = P.selectParam(param[0], val[0])
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	err = s.Err()
	if err != nil {
		log.Fatal(err)
	}

	return P
}

//evaluate text file
func (p *Param) selectParam(paramtype, value string) error {
	var err error
	switch paramtype {
	case "N":
		p.N, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic("Unable to parse N")
		}
	case "T":
		p.T, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic("Unable to parse T")
		}
	case "VisualEnabled":
		p.vEnabled, err = strconv.ParseBool(value)
		if err != nil {
			panic("Unable to parse VisualEnabled")
		}
	case "SimDuration":
		p.SimDuration, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic("Unable to parse SimDuration")
		}
	case "dropAll":
		p.dropAll, err = strconv.ParseBool(value)
		if err != nil {
			panic("Unable to parse dropAll")
		}
	}
	return err
}
