package main

import (
	"context"
	"fmt"

	"github.com/boz/ephemerald/net/client"
	"github.com/sirupsen/logrus"
)

var ()

func main() {
	log := logrus.New()
	ctx := context.Background()

	client, err := client.New(client.WithLog(log))
	if err != nil {
		log.WithError(err).Fatal("creating client")
	}

	pools, err := client.Pool().List(ctx)
	if err != nil {
		log.WithError(err).Fatal("pools")
	}

	for _, p := range pools {
		fmt.Println(p.ID)
	}

}
