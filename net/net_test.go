package net_test

import (
	"testing"

	"github.com/garyburd/redigo/redis"
	"github.com/ovrclk/cleanroom/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientServer(t *testing.T) {
	server, err := net.NewServerWithPort(0)

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

		db, err := redis.DialURL(item.URL)
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

}
