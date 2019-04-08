package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/boz/ephemerald"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/net"
	"github.com/boz/ephemerald/ui"
	"github.com/sirupsen/logrus"

	_ "github.com/boz/ephemerald/builtin/postgres"
	_ "github.com/boz/ephemerald/builtin/redis"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenPort = kingpin.Flag("port", "Listen port. Default: "+strconv.Itoa(net.DefaultPort)).Short('p').
			Default(strconv.Itoa(net.DefaultPort)).
			Int()

	configFile = kingpin.Flag("config", "config file").Short('c').
			Required().
			ExistingFile()

	logLevel = kingpin.Flag("log-level", "Log level (debug, info, warn, error).  Default: info").
			Default("info").
			Enum("debug", "info", "warn", "error")

	logFile = kingpin.Flag("log-file", "Log file.  Default: /dev/null").
		Default("/dev/null").
		OpenFile(os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)

	uiType = kingpin.Flag("ui", "UI type (tui, stream, or none). Default: tui").
		Default("tui").
		Enum("tui", "stream", "none")
)

func main() {
	kingpin.Parse()

	level, err := logrus.ParseLevel(*logLevel)
	kingpin.FatalIfError(err, "invalid log level")

	log := logrus.New()
	log.Level = level
	log.Out = *logFile

	ctx := context.Background()

	uishutdown := make(chan bool)

	var appui ui.UI

	switch *uiType {
	case "tui":
		appui, err = ui.NewTUI(uishutdown)
	case "stream":
		appui, err = ui.NewIOUI(os.Stdout)
	default:
		appui = ui.NewNoopUI()
	}
	kingpin.FatalIfError(err, "Can't start UI")

	configs, err := config.ReadFile(log, appui.Emitter(), *configFile)
	kingpin.FatalIfError(err, "invalid config file")

	pools, err := ephemerald.NewPoolSet(log, ctx, configs)
	kingpin.FatalIfError(err, "creating pools")

	builder := net.NewServerBuilder()

	builder.WithPort(*listenPort)
	builder.WithPoolSet(pools)

	server, err := builder.Create()
	if err != nil {
		pools.Stop()
		kingpin.FatalIfError(err, "can't create server")
	}

	donech := server.ServerCloseNotify()

	handleSignals(server, donech, uishutdown)

	go server.Run()

	<-donech
	appui.Stop()
}

func handleSignals(server *net.Server, donech chan bool, uishutdown chan bool) {
	go func() {
		sigch := make(chan os.Signal, 1)

		signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT)
		defer signal.Stop(sigch)

		select {
		case <-uishutdown:
			server.Close()
		case <-sigch:
			server.Close()
		case <-donech:
		}

		<-donech
	}()
}
