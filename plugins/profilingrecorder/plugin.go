package profilingrecorder

import (
	"context"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	profile "github.com/bygui86/multi-profile/v2"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/iotaledger/hive.go/core/timeutil"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
)

// PluginName is the name of the profiling plugin.
const PluginName = "ProfilingRecorder"

var (
	// Plugin is the profiling plugin.
	Plugin *node.Plugin
)

func init() {
	Plugin = node.NewPlugin(PluginName, nil, node.Disabled, run)
}

func run(*node.Plugin) {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)
	runtime.SetCPUProfileRate(5)

	profConfig := &profile.Config{
		Path:                Parameters.OutputPath,
		EnableInterruptHook: true,
		// Quiet: 			 true,
	}

	if err := daemon.BackgroundWorker(PluginName, func(ctx context.Context) {
		ticker := timeutil.NewTicker(func() {
			// TODO: always just keep x profiles around (maybe last 60min or so?)

			profConfig.Path = filepath.Join(Parameters.OutputPath, strconv.FormatInt(time.Now().Unix(), 10))

			cpuProfile := profile.CPUProfile(profConfig).Start()
			time.Sleep(10 * time.Second)
			defer cpuProfile.Stop()

			defer profile.MemProfile(profConfig).Start().Stop()
			defer profile.GoroutineProfile(profConfig).Start().Stop()
			defer profile.MutexProfile(profConfig).Start().Stop()
			defer profile.BlockProfile(profConfig).Start().Stop()
			defer profile.TraceProfile(profConfig).Start().Stop()
			defer profile.ThreadCreationProfile(profConfig).Start().Stop()
		}, 30*time.Second, ctx)

		<-ctx.Done()

		ticker.Shutdown()
		ticker.WaitForShutdown()
	}, shutdown.PriorityAnalysis); err != nil {
		panic(err)
	}
}
