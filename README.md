# ephemerald

Ephemerald manages pools of short-lived servers to be used for testing purposes.  It was built to allow paralallel integration tests which make use of (postgres, redis, vault) databases.

## Example

See [example/main.go](example/main.go) for a full working client example.

Start ephemerald server with postgres, redis enabled with a pool size of 5 for each:

```sh
$ ephemerald --pg --pg-backlog 5 --redis --redis-backlog 5
```

On the client side, configure how postgres should be initialized and reset, then open an ephemerald session:

```go
builder := net.NewClientBuilder()

builder.PG().
	WithInitialize(pgRunMigrations).
	WithReset(pgTruncateTables)

client, err := builder.Create()
```

Checkout a redis instance then open a connection to it:

```go
// checkout redis instance
ritem, err := client.Redis().Checkout()
if err != nil {
	return
}

// return to pool when done
defer client.Redis().Return(ritem)

// connect to redis instance
rconn, err := redis.DialURL(ritem.URL)
if err != nil {
	return
}
defer rconn.Close()
```

Do the same with postgres:

```go
// checkout pg instance
pitem, err := client.PG().Checkout()
if err != nil {
	return
}
// return to pool when done
defer client.PG().Return(pitem)

// connect to pg
pconn, err := sql.Open("postgres", pitem.URL)
defer pconn.Close()
```

## Status

Early days; nowhere near ready for a versioned release.

### TODO
 * Improve configuration
   * More control over container parameters
   * server-side resets (`truncate table ...`, `delete keys`, ...) and healthchecks.
   * configurable docker server connection parameters.
 * Arbitrary images (currently restricted to `postgres`, `redis`, and `vault`)
 * Nodejs client example
 * Documentation
 * Tests
 * Look into using something more portable than [koding/kite](https://github.com/koding/kite) for API.
