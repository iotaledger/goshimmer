package main

import "time"

type Param struct {
	N           int64
	vEnabled    bool
	SimDuration int64
	T           int64
}

func setParam(p *Param) {
	if p == nil {
		return
	}
	if p.N != 0 {
		N = int(p.N)
	}
	if p.T != 0 {
		SaltLifetime = time.Duration(p.T) * time.Second
	}
	if p.SimDuration != 0 {
		SimDuration = int(p.SimDuration)
	}
	vEnabled = p.vEnabled
}
