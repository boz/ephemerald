package main

import (
	"fmt"
	"os"
	"time"

	"github.com/koding/kite"
)

const (
	kiteName    = "ephemerald"
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

	channels := make(map[string]chan struct{})

	k.OnFirstRequest(func(client *kite.Client) {
		k.Log.Info("connect: client.ID: %v", client.ID)
		k.Log.Info("connect: client.Environment: %v", client.Environment)

		ch := make(chan struct{})
		channels[client.ID] = ch

		go func() {
			ticker := time.NewTicker(time.Second / 2)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_, err := client.Tell("pg.reset", &Item{ID: "x", Host: client.Environment})
					if err != nil {
						panic(err)
					}
				case <-ch:
					k.Log.Info("CLIENT %v reset", client.ID)
					return
				}
			}
		}()
	})

	k.OnDisconnect(func(client *kite.Client) {
		k.Log.Info("disconnect: client.ID: %v", client.ID)
		k.Log.Info("disconnect: client.Environment: %v", client.Environment)
		if ch, ok := channels[client.ID]; ok {
			close(ch)
		} else {
			k.Log.Warning("CAN'T FIND CHANNEL %v", client.ID)
		}
	})

	k.HandleFunc("redis.checkout", func(r *kite.Request) (interface{}, error) {
		k.Log.Info("redis.checkout: ID: %v", r.Client.ID)
		return &Item{}, nil
	})

	k.HandleFunc("redis.return", func(r *kite.Request) (interface{}, error) {
		k.Log.Info("redis.return: ID: %v", r.Client.ID)
		return nil, nil
	})

	return k
}

func NewClient() *kite.Client {

	k := kite.New(kiteName, kiteVersion)
	k.Config.Environment = "example.com"

	k.SetLogLevel(kite.DEBUG)

	k.HandleFunc("pg.reset", func(r *kite.Request) (interface{}, error) {
		item := Item{}
		r.Args.One().MustUnmarshal(&item)
		k.Log.Info("item: %v", item)
		return "sup", nil
	})

	url := fmt.Sprintf("http://localhost:%v/kite", kitePort)

	client := k.NewClient(url)

	client.Environment = "example.com"

	return client
}

func main() {
	if len(os.Args) > 0 && os.Args[1] == "server" {
		NewServer().Run()
	} else {
		client := NewClient()

		if err := client.Dial(); err != nil {
			panic(err)
		}

		for i := 0; i < 5; i++ {
			item, err := client.Tell("redis.checkout")
			if err != nil {
				panic(err)
			}
			time.Sleep(time.Second)
			_, err = client.Tell("redis.return", item)
			if err != nil {
				panic(err)
			}
			time.Sleep(time.Second)
		}

		client.Close()
		time.Sleep(time.Second)
	}
}
