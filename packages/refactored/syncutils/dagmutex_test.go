package syncutils

import (
	"fmt"
	"sync"
	"testing"

	"github.com/iotaledger/hive.go/generics/walker"
)

func TestDAGMutex(t *testing.T) {
	for i := 0; i < 5; i++ {
		dagMutex := &DAGMutex{
			consumerCounter: map[DAGMutexID]int{},
			mutexes:         map[DAGMutexID]*sync.RWMutex{},
		}

		lockableEntities := make(map[DAGMutexID]*lockableEntity)

		lockableEntities[DAGMutexID{1}] = &lockableEntity{
			id:           DAGMutexID{1},
			dependencies: nil,
			approvers:    []DAGMutexID{{2}, {3}},
		}

		lockableEntities[DAGMutexID{2}] = &lockableEntity{
			id:           DAGMutexID{2},
			dependencies: []DAGMutexID{{1}},
			approvers:    []DAGMutexID{{3}},
		}

		lockableEntities[DAGMutexID{3}] = &lockableEntity{
			id:           DAGMutexID{3},
			dependencies: []DAGMutexID{{1}, {2}},
			approvers:    nil,
		}

		lockableEntities[DAGMutexID{4}] = &lockableEntity{
			id:           DAGMutexID{4},
			dependencies: nil,
			approvers:    []DAGMutexID{{5}, {6}},
		}

		lockableEntities[DAGMutexID{5}] = &lockableEntity{
			id:           DAGMutexID{5},
			dependencies: []DAGMutexID{{4}},
			approvers:    []DAGMutexID{{6}},
		}

		lockableEntities[DAGMutexID{6}] = &lockableEntity{
			id:           DAGMutexID{6},
			dependencies: []DAGMutexID{{4}, {5}},
			approvers:    nil,
		}

		wg := new(sync.WaitGroup)

		triggerSolid := func(entityToSolidify *lockableEntity) (triggered bool) {
			dagMutex.Lock(entityToSolidify, true)
			defer dagMutex.Unlock(entityToSolidify, true)

			if entityToSolidify.solid {
				return false
			}

			for _, dependencyID := range entityToSolidify.dependencies {
				dependency := lockableEntities[dependencyID]

				if !dependency.solid {
					return false
				}
			}

			entityToSolidify.solid = true
			fmt.Println(entityToSolidify.id)

			return true
		}

		solidify := func(entityToSolidify *lockableEntity) {
			defer wg.Done()

			for it := walker.New[*lockableEntity](true).Push(entityToSolidify); it.HasNext(); entityToSolidify = it.Next() {
				if triggerSolid(entityToSolidify) {
					for _, approverID := range entityToSolidify.approvers {
						it.Push(lockableEntities[approverID])
					}
				}
			}
		}

		for _, entity := range lockableEntities {
			wg.Add(1)
			go solidify(entity)
		}
		wg.Wait()
	}

	return
}

type lockableEntity struct {
	id           DAGMutexID
	dependencies []DAGMutexID
	approvers    []DAGMutexID
	solid        bool
}

func (l *lockableEntity) DAGMutexID() DAGMutexID {
	return l.id
}

func (l *lockableEntity) DAGMutexDependencies() []DAGMutexID {
	return l.dependencies
}
