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
)

func main() {
	kingpin.Parse()

	level, err := logrus.ParseLevel(*logLevel)
	kingpin.FatalIfError(err, "invalid log level")

	log := logrus.New()
	log.Level = level

	ctx := context.Background()

	configs, err := config.Read(log, *configFile)
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

	go func() {
		sigch := make(chan os.Signal, 1)
		defer close(sigch)

		signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT)
		defer signal.Stop(sigch)

		select {
		case <-sigch:
			server.Close()
		case <-donech:
		}

		<-donech
	}()

	go server.Run()

	<-donech
}
