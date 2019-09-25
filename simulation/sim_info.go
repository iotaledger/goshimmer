package main

import (
	"sync"
	"time"
)

const (
	ACCEPTED    = 'A'
	REJECTED    = 'R'
	DROPPED     = 'D'
	OUTBOUND    = 'O'
	INCOMING    = 'I'
	ESTABLISHED = 'E'
)

type Status struct {
	timestamp int64
	opType    byte
	toNode    uint16
}

type StatusSum struct {
	outbound int
	accepted int
	incoming int
	rejected int
	dropped  int
}

type Event struct {
	eType     byte
	x         uint16
	y         uint16
	timestamp int64
}
type Link struct {
	x            uint16
	y            uint16
	tEstablished int64
	tDropped     int64
}

type StatusMap struct {
	sync.Mutex
	status map[uint16][]Status
}

func NewStatusMap() *StatusMap {
	return &StatusMap{
		status: make(map[uint16][]Status),
	}
}

func (s *StatusMap) Append(from, to uint16, op byte) {
	s.Lock()
	defer s.Unlock()
	st := Status{
		timestamp: time.Now().Unix(),
		opType:    op,
		toNode:    to,
	}
	s.status[from] = append(s.status[from], st)
}

func (s *StatusMap) GetSummary(peer uint16) (cnt StatusSum) {
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

func NewLink(x, y uint16, timestamp int64) Link {
	return Link{
		x:            x,
		y:            y,
		tEstablished: timestamp,
	}
}

func DropLink(x, y uint16, timestamp int64, list []Link) {
	for i := len(list) - 1; i > 0; i-- {
		if (list[i].x == x && list[i].y == y) ||
			(list[i].x == y && list[i].y == x) {
			list[i].tDropped = timestamp
		}
	}
}

func HasLink(target Link, list []Link) bool {
	for _, link := range list {
		if (link.x == target.x && link.y == target.y) ||
			(link.x == target.y && link.y == target.x) {
			return true
		}
	}
	return false
}
