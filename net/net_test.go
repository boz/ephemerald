package net_test

import (
	"context"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald"

	_ "github.com/boz/ephemerald/builtin/postgres"
	_ "github.com/boz/ephemerald/builtin/redis"
	"github.com/boz/ephemerald/testutil"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/net"
	"github.com/boz/ephemerald/params"
	redigo "github.com/garyburd/redigo/redis"
	"github.com/stretchr/testify/require"
)

func TestClientServer(t *testing.T) {

	log := logrus.New()
	log.Level = logrus.DebugLevel

	uie := testutil.Emitter()

	ctx := context.Background()

	configs, err := config.ReadFile(log, uie, "_testdata/config.json")
	require.NoError(t, err)

	pools, err := ephemerald.NewPoolSet(log, ctx, configs)
	require.NoError(t, err)

	server, err := net.NewServerBuilder().
		WithPort(0).
		WithPoolSet(pools).
		Create()
	if err != nil {
		pools.Stop()
		require.NoError(t, err)
	}

	donech := server.ServerCloseNotify()
	defer func() {
		<-donech
	}()
	defer server.Close()

	go server.Run()

	client, err := net.NewClientBuilder().
		WithPort(server.Port()).
		Create()
	require.NoError(t, err)

	func() {
		pset, err := client.CheckoutBatch()
		require.NoError(t, err)
		defer func() {
			require.NoError(t, client.ReturnBatch(pset))
		}()

		rparam, ok := pset["redis"]
		require.True(t, ok)
		doTestOperation(t, rparam, "multi")
	}()

	func() {
		rparam, err := client.Checkout("redis")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, client.Return("redis", rparam))
		}()
		doTestOperation(t, rparam, "single")
	}()
}

func doTestOperation(t *testing.T, rparam params.Params, message string) {
	require.NotNil(t, rparam, message)
	require.NotEmpty(t, rparam.Url, message)

	rdb, err := redigo.DialURL(rparam.Url)
	require.NoError(t, err, message)
	defer rdb.Close()

	_, err = rdb.Do("PING")
	require.NoError(t, err, message)
}
