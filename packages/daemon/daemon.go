package daemon

import (
	"sync"
)

var (
	running                  bool
	wg                       sync.WaitGroup
	ShutdownSignal           = make(chan int, 1)
	backgroundWorkers        = make([]func(), 0)
	backgroundWorkerNames    = make([]string, 0)
	runningBackgroundWorkers = make(map[string]bool)
	lock                     = sync.Mutex{}
)

func GetRunningBackgroundWorkers() []string {
	lock.Lock()

	result := make([]string, 0)
	for runningBackgroundWorker := range runningBackgroundWorkers {
		result = append(result, runningBackgroundWorker)
	}

	lock.Unlock()

	return result
}

func runBackgroundWorker(name string, backgroundWorker func()) {
	wg.Add(1)

	go func() {
		lock.Lock()
		runningBackgroundWorkers[name] = true
		lock.Unlock()

		backgroundWorker()

		lock.Lock()
		delete(runningBackgroundWorkers, name)
		lock.Unlock()

		wg.Done()
	}()
}

func BackgroundWorker(name string, handler func()) {
	lock.Lock()

	if IsRunning() {
		runBackgroundWorker(name, handler)
	} else {
		backgroundWorkerNames = append(backgroundWorkerNames, name)
		backgroundWorkers = append(backgroundWorkers, handler)
	}

	lock.Unlock()
}

func Run() {
	if !running {
		lock.Lock()

		if !running {
			ShutdownSignal = make(chan int, 1)

			running = true

			Events.Run.Trigger()

			for i, backgroundWorker := range backgroundWorkers {
				runBackgroundWorker(backgroundWorkerNames[i], backgroundWorker)
			}
		}

		lock.Unlock()
	}

	wg.Wait()
}

func Shutdown() {
	if running {
		lock.Lock()

		if running {
			close(ShutdownSignal)

			running = false

			Events.Shutdown.Trigger()
		}

		lock.Unlock()
	}
}

func IsRunning() bool {
	return running
}
