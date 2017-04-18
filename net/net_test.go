package net_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald"

	_ "github.com/boz/ephemerald/builtin/postgres"
	_ "github.com/boz/ephemerald/builtin/redis"

	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/net"
	"github.com/boz/ephemerald/params"
	redigo "github.com/garyburd/redigo/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientServer(t *testing.T) {

	log := logrus.New()
	log.Level = logrus.DebugLevel

	ctx := context.Background()

	configs, err := config.ReadPath(log, "_testdata/config.json")
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

	pset, err := client.Checkout()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, client.Return(pset))
	}()

	doTestOperation(t, pset, "main")

	for i := 0; i < 1; i++ {
		func(count int) {
			pset, err := client.Checkout()
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, client.Return(pset))
			}()
			doTestOperation(t, pset, fmt.Sprintf("child %v", count))
		}(i)
	}
}

func doTestOperation(t *testing.T, pset params.ParamSet, message string) {
	rparam, ok := pset["redis"]
	require.True(t, ok, message)
	require.NotNil(t, rparam, message)
	require.NotEmpty(t, rparam.Url, message)

	pgparam, ok := pset["postgres"]
	require.True(t, ok, message)
	require.NotNil(t, pgparam, message)
	require.NotEmpty(t, pgparam.Url, message)

	rdb, err := redigo.DialURL(rparam.Url)
	require.NoError(t, err, message)
	defer rdb.Close()

	pg, err := sql.Open("postgres", pgparam.Url)
	require.NoError(t, err, message)
	defer pg.Close()

	_, err = rdb.Do("PING")
	require.NoError(t, err, message)

	err = pg.Ping()
	require.NoError(t, err, message)
}
