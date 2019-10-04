package main

import (
	"fmt"
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
	timestamp time.Duration
}
type Link struct {
	x            uint16
	y            uint16
	tEstablished int64
	tDropped     int64
}

type Convergence struct {
	timestamp time.Duration
	counter   float64
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

func DropLink(x, y uint16, timestamp int64, list []Link) bool {
	for i := len(list) - 1; i >= 0; i-- {
		if (list[i].x == x && list[i].y == y) ||
			(list[i].x == y && list[i].y == x) {
			if list[i].tDropped == 0 {
				list[i].tDropped = timestamp
				return true
			}
			return false
		}
	}
	return false
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

func (l Link) String() string {
	result := ""
	result += fmt.Sprintf("\n%d--%d\t", l.x, l.y)
	if l.tDropped == 0 {
		return result
	}
	result += fmt.Sprintf("%d", l.tDropped-l.tEstablished)
	return result
}

func LinkSurvival(links []Link) map[int64]int {
	result := make(map[int64]int)
	for _, l := range links {
		if l.tDropped != 0 {
			result[(l.tDropped-l.tEstablished)/1000]++
		}
	}
	return result
}
