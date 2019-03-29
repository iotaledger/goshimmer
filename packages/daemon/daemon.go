package daemon

import (
    "sync"
)

var (
    running              bool
    runOnce              sync.Once
    shutdownOnce         sync.Once
    wg                   sync.WaitGroup
    installWG            sync.WaitGroup
    ShutdownSignal       = make(chan int, 1)
    backgroundWorkers    = make([]func(), 0)
    backgroundWorkerChan = make(chan func(), 10)
)

var Events = daemonEvents{
    Run: &callbackEvent{
        callbacks: map[uintptr]Callback{},
    },
    Shutdown: &callbackEvent{
        callbacks: map[uintptr]Callback{},
    },
}

func init() {
    shutdownOnce.Do(func() {})

    go func() {
        for {
            backgroundWorker := <- backgroundWorkerChan

            backgroundWorkers = append(backgroundWorkers, backgroundWorker)

            installWG.Done()

            if IsRunning() {
                runBackgroundWorker(backgroundWorker)
            }
        }
    }()
}

func runBackgroundWorker(backgroundWorker func()) {
    wg.Add(1)

    go func() {
        backgroundWorker()

        wg.Done()
    }()
}

func runBackgroundWorkers() {
    for _, backgroundWorker := range backgroundWorkers {
        runBackgroundWorker(backgroundWorker)
    }
}

func BackgroundWorker(handler func()) {
    installWG.Add(1)

    backgroundWorkerChan <- handler
}

func Run() {
    runOnce.Do(func() {
        installWG.Wait()

        ShutdownSignal = make(chan int, 1)
        running = true

        Events.Run.Trigger()

        runBackgroundWorkers()

        shutdownOnce = sync.Once{}
    })

    wg.Wait()
}

func Shutdown() {
    shutdownOnce.Do(func() {
        close(ShutdownSignal)

        running = false

        Events.Shutdown.Trigger()

        runOnce = sync.Once{}
    })
}

func IsRunning() bool {
    return running
}
