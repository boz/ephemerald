package net_test

import (
	"fmt"
	"testing"

	_ "github.com/boz/ephemerald/builtin/postgres"
	_ "github.com/boz/ephemerald/builtin/redis"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/log"
	"github.com/boz/ephemerald/net/client"
	"github.com/boz/ephemerald/net/server"
	"github.com/boz/ephemerald/poolset"
	"github.com/boz/ephemerald/scheduler"
	"github.com/boz/ephemerald/testutil"
	rredis "github.com/garyburd/redigo/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientServer(t *testing.T) {
	ctx := testutil.Context()
	node := testutil.Node(t, ctx)
	bus := testutil.Bus(t, ctx)
	defer func() {
		require.NoError(t, bus.Shutdown())
	}()
	sched := scheduler.New(ctx, bus, node)

	pset, err := poolset.New(ctx, bus, sched)
	require.NoError(t, err)
	defer pset.Shutdown()

	svr, err := server.New(server.WithAddress("localhost:0"), server.WithPoolSet(pset))
	require.NoError(t, err)
	defer svr.Close()

	addr := svr.Address()
	donech := make(chan struct{})
	go func() {
		defer close(donech)
		svr.Run()
	}()

	var rcfg config.Pool

	testutil.ReadFile(t, "../_testdata/pool.redis.yml", &rcfg)

	client, err := client.New(client.WithHost("http://"+addr),
		client.WithLog(log.FromContext(ctx)))
	require.NoError(t, err)

	// create
	pool, err := client.Pool().Create(ctx, rcfg)
	require.NoError(t, err)

	{ // get
		resp, err := client.Pool().Get(ctx, pool.ID)
		require.NoError(t, err)
		require.Equal(t, pool.ID, resp.ID)
	}

	{ // list
		resp, err := client.Pool().List(ctx)
		require.NoError(t, err)
		require.Len(t, resp, 1)
		require.Equal(t, pool.ID, resp[0].ID)
	}

	{ // checkout + release
		resp, err := client.Pool().Checkout(ctx, pool.ID)
		require.NoError(t, err)
		require.Equal(t, pool.ID, resp.PoolID)
		defer func() {
			assert.NoError(t, client.Pool().Release(ctx, resp.PoolID, resp.InstanceID))
		}()

		address := fmt.Sprintf("%v:%v", resp.Host, resp.Port)

		db := 0

		// TODO: send DB back in vars.
		// require.Contains(t, resp.Vars, "database")
		// db, err := strconv.Atoi(resp.Vars["database"])
		// require.NoError(t, err)

		conn, err := rredis.Dial("tcp", address,
			rredis.DialDatabase(db))
		require.NoError(t, err)

		defer func() {
			assert.NoError(t, conn.Close())
		}()
		require.NoError(t, conn.Send("PING"))
	}

	{ // delete
		require.NoError(t, client.Pool().Delete(ctx, pool.ID))
	}
}
