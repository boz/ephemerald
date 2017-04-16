package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"

	_ "github.com/lib/pq"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/builtin/pg"
	"github.com/boz/ephemerald/net"
	"github.com/garyburd/redigo/redis"
)

func main() {
	log := logrus.New()

	builder := net.NewClientBuilder()
	builder.PG(func(b *pg.ClientBuilder) {
		b.WithInitialize(pgRunMigrations)
		b.WithReset(pgTruncateTables)
	})

	client, err := builder.Create()
	if err != nil {
		log.WithError(err).Fatal("can't create client")
	}

	var wg sync.WaitGroup

	passed := uint32(0)
	failed := uint32(0)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// checkout redis instance
			ritem, err := client.Redis().Checkout()
			if err != nil {
				return
			}
			defer client.Redis().Return(ritem)

			// connect to redis instance
			rconn, err := redis.DialURL(ritem.URL)
			if err != nil {
				return
			}
			defer rconn.Close()

			// checkout pg instance
			pitem, err := client.PG().Checkout()
			if err != nil {
				return
			}
			defer client.PG().Return(pitem)

			// connect to pg
			pconn, err := sql.Open("postgres", pitem.URL)
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
	return nil
}

func pgRunMigrations(_ context.Context, item *pg.Item) error {
	db, err := sql.Open("postgres", item.URL)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			UNIQUE(name)
		);
		`)

	return err
}

func pgTruncateTables(_ context.Context, item *pg.Item) error {
	db, err := sql.Open("postgres", item.URL)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("TRUNCATE TABLE users;")
	return err
}
