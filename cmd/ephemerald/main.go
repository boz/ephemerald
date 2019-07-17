package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/log"
	"github.com/boz/ephemerald/net"
	"github.com/boz/ephemerald/net/server"
	"github.com/boz/ephemerald/node"
	"github.com/boz/ephemerald/poolset"
	"github.com/boz/ephemerald/pubsub"
	"github.com/boz/ephemerald/scheduler"
	"github.com/sirupsen/logrus"

	_ "github.com/boz/ephemerald/builtin/postgres"
	_ "github.com/boz/ephemerald/builtin/redis"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenAddress = kingpin.Flag("address", "Listen address. Default: "+net.DefaultListenAddress).Short('a').
			Envar("EPHEMERALD_LISTEN_ADDRESS").
			Default(net.DefaultListenAddress).
			String()

	poolFiles = kingpin.Flag("pool", "pool config file").Short('p').
			ExistingFiles()

	logLevel = kingpin.Flag("log-level", "Log level (debug, info, warn, error).  Default: info").
			Envar("EPHEMERALD_LOG_LEVEL").
			Default("info").
			Enum("debug", "info", "warn", "error")

	logFile = kingpin.Flag("log-file", "Log file.  Default: /dev/null").
		Envar("EPHEMERALD_LOG_FILE").
		Default("/dev/null").
		OpenFile(os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
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

	opts := []server.Opt{
		server.WithAddress(*listenAddress),
		server.WithPoolSet(pset),
	}

	sdonech := make(chan struct{})
	server, err := server.New(opts...)
	if err != nil {
		kingpin.Errorf("can't create server: %v", err)
		goto done
	}

	go func() {
		defer close(sdonech)
		if err := server.Run(); err != nil {
			log.WithError(err).Warn("server run")
		}
	}()

	select {
	case <-sdonech:
		log.Info("server done")
	case <-stopch:
		log.Info("shutdown requested")
		server.Close()
	}

done:

	log.Info("shutting down pools...")
	pset.Shutdown()
	<-pset.Done()

	log.Info("shutting down UI...")
	cancel()

	if err := bus.Shutdown(); err != nil {
		log.WithError(err).Warn("bus shutdown")
	}

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
