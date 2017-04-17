package net_test

import (
	"database/sql"
	"testing"

	"github.com/boz/ephemerald/builtin/pg"
	"github.com/boz/ephemerald/builtin/redis"
	"github.com/boz/ephemerald/net"
	redigo "github.com/garyburd/redigo/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientServer(t *testing.T) {

	builder := net.NewServerBuilder().WithPort(0)
	builder.PG().WithSize(2).WithLabel("test", "net.TestClientServer")
	builder.Redis().WithSize(2).WithLabel("test", "net.TestClientServer")

	server, err := builder.Create()

	require.NoError(t, err)

	readych := server.ServerReadyNotify()
	donech := server.ServerCloseNotify()
	defer func() {
		<-donech
	}()
	defer server.Close()

	go server.Run()
	<-readych

	client, err := net.NewClientBuilder().
		WithPort(server.Port()).
		Create()
	require.NoError(t, err)

	{
		item, err := client.Redis().Checkout()
		require.NoError(t, err)
		defer func() {
			assert.NoError(t, client.Redis().Return(item))
		}()

		db, err := redigo.DialURL(item.URL)
		require.NoError(t, err)

		_, err = db.Do("PING")
		require.NoError(t, err)
	}
	{
		item, err := client.PG().Checkout()
		require.NoError(t, err)
		defer func() {
			assert.NoError(t, client.PG().Return(item))
		}()
	}

	for i := 0; i < 2; i++ {
		func() {
			ri, db := getRedis(t, client)
			defer func() {
				client.Redis().Return(ri)
			}()

			pi, pq := getPG(t, client)
			defer func() {
				client.PG().Return(pi)
			}()
			defer func() {
				assert.NoError(t, pq.Close())
			}()

			for i := 0; i < 2; i++ {
				{
					_, err := db.Do("PING")
					assert.NoError(t, err)
				}
				{
					assert.NoError(t, pq.Ping())
				}
			}

		}()
	}
}

func getRedis(t *testing.T, c *net.Client) (*redis.Item, redigo.Conn) {
	i, err := c.Redis().Checkout()
	require.NoError(t, err)

	db, err := redigo.DialURL(i.URL)
	require.NoError(t, err)

	return i, db
}

func getPG(t *testing.T, c *net.Client) (*pg.Item, *sql.DB) {
	i, err := c.PG().Checkout()
	require.NoError(t, err)

	db, err := sql.Open("postgres", i.URL)
	require.NoError(t, err)

	return i, db
}
