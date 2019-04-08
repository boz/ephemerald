package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/boz/ephemerald/builtin/postgres"
	_ "github.com/boz/ephemerald/builtin/redis"
	_ "github.com/lib/pq"

	"github.com/boz/ephemerald/net"
	"github.com/garyburd/redigo/redis"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()

	builder := net.NewClientBuilder()

	client, err := builder.Create()
	if err != nil {
		log.WithError(err).Fatal("can't create client")
	}

	var wg sync.WaitGroup

	passed := uint32(0)
	failed := uint32(0)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			secs := time.Duration(rand.Intn(5)) * time.Second
			time.Sleep(secs)

			var redisUrl string
			var pgUrl string

			if i%2 == 0 {
				// multi checkout
				items, err := client.CheckoutBatch()
				if err != nil {
					log.WithError(err).Error("checkout multi")
					return
				}
				defer func() {
					if err := client.ReturnBatch(items); err != nil {
						log.WithError(err).Error("return multi")
					}
				}()

				pgUrl = items["postgres"].Url
				redisUrl = items["redis"].Url

			} else {
				// single checkout

				rparam, err := client.Checkout("redis")
				if err != nil {
					log.WithError(err).Error("checkout redis")
					return
				}
				defer func() {
					if err := client.Return("redis", rparam); err != nil {
						log.WithError(err).Error("return redis")
					}
				}()

				redisUrl = rparam.Url

				pgparam, err := client.Checkout("postgres")
				if err != nil {
					log.WithError(err).Error("checkout postgres")
					return
				}
				defer func() {
					if err := client.Return("postgres", rparam); err != nil {
						log.WithError(err).Error("return postgres")
					}
				}()

				pgUrl = pgparam.Url
			}

			// connect to redis instance
			rconn, err := redis.DialURL(redisUrl)
			if err != nil {
				log.WithError(err).Error("dialing redis")
				return
			}
			defer rconn.Close()

			// connect to pg
			pconn, err := sql.Open("postgres", pgUrl)
			if err != nil {
				log.WithError(err).Error("dialing postgres")
				return
			}
			defer pconn.Close()

			// run tests
			err = runTests(rconn, pconn)
			if err != nil {
				atomic.AddUint32(&failed, uint32(1))
				log.WithError(err).Error("test failed")
			} else {
				atomic.AddUint32(&passed, uint32(1))
				log.Info("test passed")
			}
		}()
	}
	wg.Wait()

	total := passed + failed

	log.WithField("total", total).
		WithField("passed", passed).
		WithField("failed", failed).
		Infof("Tests complete")
}

func runTests(rconn redis.Conn, pconn *sql.DB) error {
	res, err := pconn.Exec("INSERT INTO users (name) VALUES($1);", "foo")
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("invalid rows affected: %v != %v", 1, count)
	}

	secs := time.Duration(rand.Intn(5)*500) * time.Millisecond
	time.Sleep(secs)

	return nil
}
