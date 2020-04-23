package httpserver

import (
	"errors"
	"net/http"
	"time"

	"github.com/gobuffalo/packr/v2"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/labstack/echo"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
)

var (
	log    *logger.Logger
	engine *echo.Echo
)

const name = "Analysis HTTP Server"

var assetsBox = packr.New("Assets", "./static")

func Configure() {
	log = logger.NewLogger(name)

	engine = echo.New()
	engine.HideBanner = true

	// we only need this special flag, because we always keep a packed box in the same directory
	if config.Node.GetBool(CfgDev) {
		engine.Static("/static", "./plugins/analysis/webinterface/httpserver/static")
		engine.File("/", "./plugins/analysis/webinterface/httpserver/static/index.html")
	} else {
		for _, res := range assetsBox.List() {
			engine.GET("/static/"+res, echo.WrapHandler(http.StripPrefix("/static", http.FileServer(assetsBox))))
		}
		engine.GET("/", index)
	}

	engine.GET("/datastream", echo.WrapHandler(websocket.Handler(dataStream)))
}

func Run() {
	log.Infof("Starting %s ...", name)
	if err := daemon.BackgroundWorker(name, start, shutdown.PriorityAnalysis); err != nil {
		log.Errorf("Error starting as daemon: %s", err)
	}
}

func start(shutdownSignal <-chan struct{}) {
	stopped := make(chan struct{})
	bindAddr := config.Node.GetString(CfgBindAddress)
	go func() {
		log.Infof("Started %s: http://%s", name, bindAddr)
		if err := engine.Start(bindAddr); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Errorf("Error serving: %s", err)
			}
			close(stopped)
		}
	}()

	select {
	case <-shutdownSignal:
	case <-stopped:
	}

	log.Infof("Stopping %s ...", name)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := engine.Shutdown(ctx); err != nil {
		log.Errorf("Error stopping: %s", err)
	}
	log.Info("Stopping %s ... done", name)
}

func index(e echo.Context) error {
	indexHTML, err := assetsBox.Find("index.html")
	if err != nil {
		return err
	}
	return e.HTMLBlob(http.StatusOK, indexHTML)
}
