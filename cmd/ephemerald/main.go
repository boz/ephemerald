package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/log"
	"github.com/boz/ephemerald/net"
	"github.com/boz/ephemerald/node"
	"github.com/boz/ephemerald/poolset"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/scheduler"
	"github.com/boz/ephemerald/ui"
	"github.com/sirupsen/logrus"

	_ "github.com/boz/ephemerald/builtin/postgres"
	_ "github.com/boz/ephemerald/builtin/redis"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenPort = kingpin.Flag("port", "Listen port. Default: "+strconv.Itoa(net.DefaultPort)).Short('p').
			Envar("EPHEMERALD_PORT").
			Default(strconv.Itoa(net.DefaultPort)).
			Int()

	poolFiles = kingpin.Flag("pool", "pool config file").Short('P').
			ExistingFiles()

	logLevel = kingpin.Flag("log-level", "Log level (debug, info, warn, error).  Default: info").
			Envar("EPHEMERALD_LOG_LEVEL").
			Default("info").
			Enum("debug", "info", "warn", "error")

	logFile = kingpin.Flag("log-file", "Log file.  Default: /dev/null").
		Envar("EPHEMERALD_LOG_FILE").
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

	l := log.Default()
	l.SetLevel(level)
	l.SetOutput(*logFile)

	ctx, cancel := context.WithCancel(context.Background())
	ctx = log.NewContext(ctx, l)

	log := l.WithField("cmp", "main")

	stopch := handleSignals(ctx, cancel)

	bus, err := pubsub.NewBus(ctx)
	kingpin.FatalIfError(err, "pubsub bus")

	node, err := node.NewFromEnv(ctx)
	kingpin.FatalIfError(err, "node")

	scheduler := scheduler.New(ctx, bus, node)

	pset, err := poolset.New(ctx, bus, scheduler)
	kingpin.FatalIfError(err, "poolset")

	ui, err := ui.NewEVLog(ctx, bus, os.Stdout)
	kingpin.FatalIfError(err, "ui")

	for _, pfile := range *poolFiles {
		var pcfg config.Pool

		if err := config.ReadFile(pfile, &pcfg); err != nil {
			log.WithError(err).WithField("pool-file", pfile).Error("reading pool config")
			kingpin.Errorf("error reading pool config %v: %v", pfile, err)
			continue
		}

		_, err := pset.Create(ctx, pcfg)
		if err != nil {
			log.WithError(err).WithField("pool-file", pfile).Error("creating pool")
			kingpin.Errorf("error creating pool %v: %v", pcfg.Name, err)
		}
	}

	builder := net.NewServerBuilder()
	builder.WithPort(*listenPort)

	server, err := builder.Create()
	if err != nil {
		log.WithError(err).Error("creating server")
		kingpin.FatalIfError(err, "can't create server")
	}

	sdonech := server.ServerCloseNotify()

	go server.Run()

	select {
	case <-sdonech:
		log.Info("server done")
	case <-stopch:
		log.Info("shutdown requested")
		server.Close()
	}

	log.Info("shutting down pools...")
	pset.Shutdown()
	<-pset.Done()

	log.Info("shutting down UI...")
	ui.Stop()
	cancel()
	bus.Shutdown()

	<-stopch
}

func handleSignals(ctx context.Context, cancel context.CancelFunc) <-chan struct{} {
	donech := make(chan struct{})
	go func() {
		defer close(donech)

		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)
		defer signal.Stop(sigch)

		select {
		case <-ctx.Done():
		case <-sigch:
		}
	}()
	return donech
}
