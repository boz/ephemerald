package main

import (
	"fmt"
	"os"

	"github.com/cleanroom/builtin/redis"
	"github.com/koding/kite"
)

const (
	kiteName    = "cleanroom"
	kiteVersion = "0.0.1"
	kitePort    = 6000
)

type Item struct {
	ID   string
	Host string
}

func NewServer() *kite.Kite {
	k := kite.New(kiteName, kiteVersion)

	k.SetLogLevel(kite.DEBUG)

	k.Config.Port = kitePort
	k.Config.DisableAuthentication = true

	k.HandleFunc("connect", func(r *kite.Request) (interface{}, error) {
		var services []string

		for _, arg := range r.Args.MustSlice() {
			services = append(services, arg.MustString())
		}

		for svc := range services {
			switch svc {
			case "redis":

			}
		}

		return nil, nil
	})

	var pool redis.Pool

	k.HandleFunc("redis/checkout", func(r *kite.Request) (interface{}, error) {
		return pool.Checkout()
	})

	k.HandleFunc("redis/return", func(r *kite.Request) (interface{}, error) {
	})

	k.HandleFunc("do-reset", func(r *kite.Request) (interface{}, error) {
		k.Log.Info("do-reset")
		return "ok", nil
	})
	return k
}

func NewClient() *kite.Client {

	k := kite.New(kiteName, kiteVersion)

	k.SetLogLevel(kite.DEBUG)

	url := fmt.Sprintf("http://localhost:%v/kite", kitePort)

	k.HandleFunc("reset", func(r *kite.Request) (interface{}, error) {
		k.Log.Info("received request: %v", r.Args)

		item := Item{}
		//err := json.Unmarshal(r.Args.Raw, &item)
		err := r.Args.One().Unmarshal(&item)

		k.Log.Info("item: %v; err: %v", item, err)

		return "sup", nil
	})

	client := k.NewClient(url)

	return client
}

func main() {
	if os.Args[1] == "server" {
		NewServer().Run()
	} else {
		client := NewClient()

		if err := client.Dial(); err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}

		response, err := client.Tell("redis/checkout", "hello")
		fmt.Printf("response: %v err: %v\n", response, err)

	}
}
