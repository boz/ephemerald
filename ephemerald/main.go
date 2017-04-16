package main

import (
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/net"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenPort = kingpin.Flag("port", "Listen port").Short('p').
			Default(strconv.Itoa(net.DefaultPort)).
			Int()

	pgEnable = kingpin.Flag("pg", "Enable postgres").
			Default("true").
			Bool()

	pgBacklog = kingpin.Flag("pg-backlog", "Postgres backlog size").
			Default("5").
			Int()

	redisEnable = kingpin.Flag("redis", "Enable redis").
			Default("true").
			Bool()

	redisBacklog = kingpin.Flag("redis-backlog", "Redis backlog size").
			Default("5").
			Int()

	logLevel = kingpin.Flag("log-level", "Log level").
			Default("info").
			Enum("debug", "info", "error", "warn")
)

func main() {
	kingpin.Parse()

	builder := net.NewServerBuilder()

	builder.WithPort(*listenPort)

	level, err := logrus.ParseLevel(*logLevel)
	kingpin.FatalIfError(err, "invalid log level")

	if *pgEnable {
		builder.PG().
			WithSize(*pgBacklog).WithLogLevel(level)
	}

	if *redisEnable {
		builder.Redis().
			WithSize(*redisBacklog).WithLogLevel(level)
	}

	server, err := builder.Create()
	kingpin.FatalIfError(err, "can't create server")

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
