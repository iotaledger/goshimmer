package daemon

import (
    "sync"
)

var (
    running           bool
    wg                sync.WaitGroup
    ShutdownSignal    = make(chan int, 1)
    backgroundWorkers = make([]func(), 0)
    lock              = sync.Mutex{}
)

func runBackgroundWorker(backgroundWorker func()) {
    wg.Add(1)

    go func() {
        backgroundWorker()

        wg.Done()
    }()
}

func BackgroundWorker(handler func()) {
    lock.Lock()

    if IsRunning() {
        runBackgroundWorker(handler)
    } else {
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

            for _, backgroundWorker := range backgroundWorkers {
                runBackgroundWorker(backgroundWorker)
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
