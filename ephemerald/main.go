package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/net"
	"github.com/boz/ephemerald/ui"

	_ "github.com/boz/ephemerald/builtin/postgres"
	_ "github.com/boz/ephemerald/builtin/redis"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenPort = kingpin.Flag("port", "Listen port").Short('p').
			Default(strconv.Itoa(net.DefaultPort)).
			Int()

	configFile = kingpin.Flag("config", "config file").Short('f').
			Required().
			File()

	logLevel = kingpin.Flag("log-level", "Log level").
			Default("info").
			Enum("debug", "info", "error", "warn")

	logFile = kingpin.Flag("log-file", "Log file").
		Default("/dev/null").
		OpenFile(os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)

	useGUI = kingpin.Flag("gui", "terminal gui output").
		Default("true").
		Bool()
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

	if *useGUI {
		appui, err = ui.NewTUI(uishutdown)
	} else {
		appui, err = ui.NewIOUI(os.Stdout)
	}
	kingpin.FatalIfError(err, "Can't start UI")

	configs, err := config.Read(log, appui.Emitter(), *configFile)
	(*configFile).Close()
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
