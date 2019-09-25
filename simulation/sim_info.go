package main

import (
	"sync"
	"time"
)

const (
	ACCEPTED = 'A'
	REJECTED = 'R'
	DROPPED  = 'D'
	OUTBOUND = 'O'
	INCOMING = 'I'
)

type Status struct {
	timestamp int64
	opType    byte
	toNode    int
}

type StatusSum struct {
	outbound int
	accepted int
	incoming int
	rejected int
	dropped  int
}

type StatusMap struct {
	sync.Mutex
	status map[int][]Status
}

func NewStatusMap() *StatusMap {
	return &StatusMap{
		status: make(map[int][]Status),
	}
}

func (s *StatusMap) Append(from, to int, op byte) {
	s.Lock()
	defer s.Unlock()
	st := Status{
		timestamp: time.Now().Unix(),
		opType:    op,
		toNode:    to,
	}
	s.status[from] = append(s.status[from], st)
}

func (s *StatusMap) GetSummary(peer int) (cnt StatusSum) {
	s.Lock()
	defer s.Unlock()
	for _, t := range s.status[peer] {
		switch t.opType {
		case ACCEPTED:
			cnt.accepted++
		case REJECTED:
			cnt.rejected++
		case DROPPED:
			cnt.dropped++
		case OUTBOUND:
			cnt.outbound++
		case INCOMING:
			cnt.incoming++
		}
	}
	return cnt
}
