# ephemerald

Ephemerald manages pools of short-lived servers to be used for testing purposes.  It was built to allow paralallel integration tests.

[![asciicast](https://asciinema.org/a/117629.png)](https://asciinema.org/a/117629)

It has [REST API](#api) for accessing server instances from any language and comes with a built-in [go client](net/client.go). See the [examples](_example/) directory for example configurations and client usage.

The ephemerald server can run on a remote host; container connection parameters are rewritten so that the client
connects to the right place.  This way ephemerald can run on a large server and be used from less powerful
development machines.

* [Running](#running)
* [Configuration](#building)
  * [Params](#params)
  * [Container](#container)
  * [Lifecycle Actions](#lifecycle-actions)
    * [noop](#noop)
    * [exec](#exec)
    * [http.get](#httpget)
    * [tcp.connect](#tcpconnect)
    * [postgres.exec](#postgresexec)
    * [postgres.ping](#postgresping)
    * [postgres.truncate](#postgrestruncate)
    * [redis.exec](#redisexec)
    * [redis.ping](#redisping)
    * [redis.truncate](#redistruncate)
* [API](#api)
  * [Checkout](#checkout)
  * [Return](#return)
  * [Batch Checkout](#batch-checkout)
  * [Batch Return](#batch-return)
* [Building](#building)
* [Installing](#installing)
  * [Homebrew](#homebrew)
  * [Binary](#binary)
  * [Source](#source)
* [TODO](#todo)

## Running

To run the server, supply a configuration file:

```sh
$ ephemerald -c config.yaml
```

Press Q to quit the server.

### Flags

 * `--help` print help message.
 * `-p <port>` changes the listen port.  Defaults to 6000
 * `--ui stream` will dump the event steam to the console in lieu of a curses-esque UI.
 * `--ui none` will not print any UI information (useful with `--log-file /dev/stdout`)
 * `--log-file <path>` write logs to file at `path`.  Defaults to `/dev/null`
 * `--log-level <level>` log level.  defaults to `info`.  Options are `debug`,`info`,`warn`,`error`

Note: use Ctrl-C to stop the server wen not in `--ui tui` mode (`SIGINT`,`SIGQUIT` always work too)

For example, to see only log messages (at debug level) use:

```sh
$ ephememerald --ui none --log-level debug --log-file /dev/stdout -c config.yaml
```

## Configuration

Container pools are configured in a yaml (or json) file.  Each pool has options for the container parameters and
for lifecycle actions.

The following configuration creates a single pool called "pg" which maintains five containers from the "postgres" image and
exposes port 5432 to clients.  See the [`params`](#params) and [`actions`](#lifecycle-actions) below for documentation on those fields.

```yaml
pools:
  pg:
    image: postgres
    size: 5
    port: 5432
    params:
      username: postgres
      database: postgres
      url: postgres://{{.Username}}:@{{.Hostname}}:{{.Port}}/{{.Database}}?sslmode=disable
    actions:
      live:
        type: postgres.ping
        retries: 10
        delay:   50ms
      init:
        type:    exec
        path:    make
        args:    [ 'db:migrate' ]
        env:     [ 'DATABASE_URL={{.Url}}' ]
        timeout: 10s
      reset:
        type: postgres.truncate
```

See [example/config.yaml](_example/config.yaml) for a full working configuration.

### Params

The `params` entry allows for declaring parameters needed for connecting to the service.  There are three fields
with arbitrary values: `username`, `password`, `database`.

The `url` can be a golang template and will be executed with access to the following fields:

Name | Value
--- | ---
Hostname | The hostname that the container can be connected at
Port | The (automatically-generated) port number that is mapped to the exposed container port
Username | The `username` field declared in `params`
Password | The `password` field declared in `params`
Database | The `database` field declared in `params`

A `params` section for postgres may look like this:

```yaml
username: postgres
database: postgres
url: postgres://{{.Username}}:{{.Password}}@{{.Hostname}}:{{.Port}}/{{.Database}}?sslmode=disable
```

### Container

The `container` section is passed-through to docker when creating the container.  The available
options are:

 * labels
 * env
 * cmd
 * volumes
 * entrypoint
 * user
 * capadd
 * capdrop

### Lifecycle Actions

There are three lifecycle actions: `live`, `init`, and `reset`.

 * `live` is used to determine when the container is ready to be used.
 * `init` is used to init the container (run migrations, etc...)
 * `reset` may be used to revert the container to a state where it can be used again.

All of them are optional (though `live` should be used).  If `reset` is not given,
the container will be killed and a new one will be created to replace it.

Each action has, at a minimum, the following three parameters:

Name | Default | Description
---  | --- | ---
retries | 3 | number of times to retry the action
timeout | 1s | amount of time to allow for the action
delay | 500ms | amount of time to delay before retrying

`timeout` and `delay` are durations; they must have a unit suffix as described [here](https://golang.org/pkg/time/#ParseDuration).

Note: actions may have different defaults for these fields.

#### noop

Does nothing.  Useful as the `reset` action so that a container is always reused.

#### exec

Execute a command on the host operating system.  Useful for running migrations to init a database.

Extra Parameters:

Name | Default | Description
--- | --- | ---
command| `""` | command to execute
args | `[]` | command-line arguments
env | `[]` | environment variables
dir | `""` | directory to execute in.

The `env` entries may be templates with access to the same fields as the [`params`](#params) url template.  Additionally,
the following environment variables are always set:

 * `EPHEMERALD_ID`
 * `EPHEMERALD_HOSTNAME`
 * `EPHEMERALD_PORT`
 * `EPHEMERALD_USERNAME`
 * `EPHEMERALD_PASSWORD`
 * `EPHEMERALD_DATABASE`
 * `EPHEMERALD_URL`

If `dir` is not set, the working directory of the server isused.

#### http.get

Run a HTTP GET request.

Extra Parameters:

Name | Default | Description
--- | --- | ---
url | `""` | url to request

If `url` is blank, the `url` from the [`params`](#params) is used.

If `url` is not blank, it may be a template which has access to the same fields that [`params`](#params) url template does.

#### tcp.connect

Connect to the exposed container port over TCP.

#### postgres.exec

Executes a query on the database.

Extra Parameters:

Name | Default | Description
--- | --- | ---
command | `"SELECT 1=1"` | query to execute
args | `[]` | values to be escaped with positional arguments in `command`.

Example:

```yaml
type:     postgres.exec
command: 'INSERT INTO users (name) VALUES ($1)'
args:    "Robert'); DROP TABLE STUDENTS;--"
```

#### postgres.ping

Pings the database.  Useful for live.

#### postgres.truncate

Runs `TRUNCATE TABLE x CASCADE` for all tables `x`.

Extra Parameters:

Name | Default | Description
--- | --- | ---
exclude | `[]` | an array of table names to not truncate (eg migration versions)

#### redis.exec

Execute a redis command.

Extra Parameters:

Name | Default | Description
--- | --- | ---
command | `"PING"` | redis command to execute

#### redis.ping

This is an alias for `redis.exec`.

#### redis.truncate

This is an alias for `redis.exec` with a default command of `"FLUSHALL"`.

## API

There is a REST API for clients to checkout and return items from one or more pools.

### Checkout

`POST /checkout/{pool}` checks out an instance from the given pool and returns
that instance's parameters:

```sh
$ curl -s -XPOST localhost:6000/checkout/postgres | jq
{
  "id":"8482c266192f013346d03f71b2aa6d4b647909e3502ac525039bdd0fe9fcac30",
  "hostname":"localhost",
  "port":"34031",
  "username":"postgres",
  "database":"postgres",
  "url":"postgres://postgres:@localhost:34031/postgres?sslmode=disable"
}
```

### Return

`DELETE /return/{pool}/{id}` returns the instance given by `id` to the pool `pool`:

```sh
$ curl -s -XDELETE localhost:6000/return/postgres/8482c266192f013346d03f71b2aa6d4b647909e3502ac525039bdd0fe9fcac30
```

### Batch Checkout

`POST /checkout` checks out an instance from every configured pool.

```sh
$ curl -s -XPOST localhost:6000/checkout | tee checkout.json | jq
{
  "postgres": {
    "id": "2dedf5dbe9cc8d7a0cd71ed75455c7310db79aea44925562b82c01b959d85e7e",
    "hostname": "localhost",
    "port": "34023",
    "username": "postgres",
    "database": "postgres",
    "url": "postgres://postgres:@localhost:34023/postgres?sslmode=disable"
  },
  "redis": {
    "id": "a8dbf5043c7145510f48ccffa6f1e20b9f2c8140dda73d567a29dc2ec823ca46",
    "hostname": "localhost",
    "port": "34019",
    "database": "0",
    "url": "redis://localhost:34019/0"
  },
  "vault": {
    "id": "11f4752d5e0b762c65b05809c9500a6e0a20ee4a79b861638a084adf77dbfb78",
    "hostname": "localhost",
    "port": "34021",
    "url": "http://localhost:34021"
  }
}
```

### Batch Return

`DELETE /return` returns multiple instances at once.  Meant to be used with [batch checkout](#batch-checkout).

```sh
$ cat checkout.json
{
  "postgres": {
    "id": "2dedf5dbe9cc8d7a0cd71ed75455c7310db79aea44925562b82c01b959d85e7e"
  },
  "redis": {
    "id": "a8dbf5043c7145510f48ccffa6f1e20b9f2c8140dda73d567a29dc2ec823ca46"
  },
  "vault": {
    "id": "11f4752d5e0b762c65b05809c9500a6e0a20ee4a79b861638a084adf77dbfb78"
  }
}
$ curl -XDELETE -H'Content-Type: application/json' -d @checkout.json localhost:6000/return
```

Note that the complete response from [batch checkout](#batch-checkout) may be sent.  The only requirement is the `id` field for each pool instance.

## Building

```sh
$ govendor get -d github.com/boz/ephemerald/...
$ cd $GOPATH/src/github.com/boz/ephemerald
$ make server example
```
Run the example server and client in separate terminals

```sh
$ ./ephemerald/ephemerald -c _example/config.yaml
```

```sh
$ ./_example/example
```

## Installing

### Source

Follow the [building](#building) steps then run `make install`:

```sh
$ make install
```

### Binary

Download the [latest release](https://github.com/boz/ephemerald/releases/latest) for your system.  Unpack the archive and put the binary in your path.

```sh
$ release="https://github.com/boz/ephemerald/releases/download/v0.3.1/ephemerald_Linux_x86_64.tar.gz"
$ curl -L "$release" | tar -C /tmp -zxv
$ /tmp/ephemerald -c config.yaml
```

### Homebrew

```
$ brew install boz/repo/ephemerald
```

## TODO

 * Configuration
   * Current parsing is a disaster
   * Allow yaml
   * Allow built-in defaults (postgres, redis, etc...)
 * Polish/Optimize/Cleanup/Refactor UI.
 * Re-add remote actions (websockets API)
 * Clients: nodejs, ruby, python, etc...
 * Documentation
 * Tests
