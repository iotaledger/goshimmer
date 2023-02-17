package profilingrecorder

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	profile "github.com/bygui86/multi-profile/v2"
	"github.com/zyedidia/generic/cache"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/runtime/timeutil"
)

// PluginName is the name of the profiling plugin.
const PluginName = "ProfilingRecorder"

var (
	// Plugin is the profiling plugin.
	Plugin *node.Plugin
	paths  *cache.Cache[string, types.Empty]
)

func init() {
	Plugin = node.NewPlugin(PluginName, nil, node.Disabled, run)
}

func run(*node.Plugin) {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)
	runtime.SetCPUProfileRate(5)

	profConfig := &profile.Config{
		Path:  Parameters.OutputPath,
		Quiet: true,
	}

	paths = cache.New[string, types.Empty](Parameters.Capacity)
	paths.SetEvictCallback(func(key string, value types.Empty) {
		err := os.RemoveAll(key)
		if err != nil {
			Plugin.LogError("Error while removing profiling data: ", err)
		}
	})

	if err := daemon.BackgroundWorker(PluginName, func(ctx context.Context) {
		ticker := timeutil.NewTicker(func() {
			path := filepath.Join(Parameters.OutputPath, strconv.FormatInt(time.Now().Unix(), 10))
			paths.Put(path, types.Void)
			profConfig.Path = path

			cpuProfile := profile.CPUProfile(profConfig).Start()
			memProfile := profile.MemProfile(profConfig).Start()
			time.Sleep(Parameters.Duration)

			defer cpuProfile.Stop()
			defer memProfile.Stop()
			defer profile.GoroutineProfile(profConfig).Start().Stop()
			defer profile.MutexProfile(profConfig).Start().Stop()
			defer profile.BlockProfile(profConfig).Start().Stop()
			defer profile.TraceProfile(profConfig).Start().Stop()
			defer profile.ThreadCreationProfile(profConfig).Start().Stop()
		}, Parameters.Interval, ctx)

		<-ctx.Done()

		ticker.Shutdown()
		ticker.WaitForShutdown()
	}, shutdown.PriorityProfiling); err != nil {
		panic(err)
	}
}
