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
	"github.com/boz/ephemerald/version"
	"github.com/sirupsen/logrus"

	_ "github.com/boz/ephemerald/builtin/postgres"
	_ "github.com/boz/ephemerald/builtin/redis"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenAddress = kingpin.Flag("listen-address", "Listen address. Default: "+net.DefaultListenAddress).
			Short('l').
			Default(net.DefaultListenAddress).
			String()

	poolFiles = kingpin.Flag("pool", "pool config file").
			Short('p').
			ExistingFiles()

	flagLogLevel = kingpin.Flag("log-level", "Log level (debug, info, warn, error).  Default: info").
			Short('v').
			Default("info").
			Enum("debug", "info", "warn", "error")

	flagLogFile = kingpin.Flag("log-file", "Log file.  Default: /dev/stderr").
			Default("/dev/stderr").
			String()
)

func main() {

	kingpin.CommandLine.Author("Adam Bozanich <adam.boz@gmail.com>")
	kingpin.CommandLine.Version(version.String())
	kingpin.HelpFlag.Short('h')
	kingpin.CommandLine.DefaultEnvars()

	kingpin.Parse()

	log, ctx := createLog()

	ctx, cancel := context.WithCancel(ctx)

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
		server.WithLog(log),
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

func createLog() (logrus.FieldLogger, context.Context) {

	level, err := logrus.ParseLevel(*flagLogLevel)
	kingpin.FatalIfError(err, "Invalid log level")

	file, err := os.OpenFile(*flagLogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	kingpin.FatalIfError(err, "Error opening log file")

	logger := logrus.New()
	logger.SetLevel(level)
	logger.SetOutput(file)

	return logger, log.NewContext(context.Background(), logger)
}
