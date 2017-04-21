# ephemerald

Ephemerald manages pools of short-lived servers to be used for testing purposes.  It was built to allow paralallel integration tests which make use of (postgres, redis, vault) databases.

[![asciicast](https://asciinema.org/a/4gicxubpag6ltqkafvznmcdu3.png)](https://asciinema.org/a/4gicxubpag6ltqkafvznmcdu3)

* [Building](#building)
* [Running](#building)
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
* [TODO](#todo)

## Building

```sh
$ govendor get -d github.com/boz/ephemerald/...
$ cd $GOPATH/src/github.com/boz/ephemerald
$ make server example
```

## Running

To run the server, supply a configuration file:

```sh
$ ./ephemerald/ephemerald -f ./example/config.json
```

Run the [example client](example/main.go) in another terminal

```sh
$ ./example/example
```

Press Ctrl-C to quit the server.

## Configuration

Container pools are configured in a json file.  Each pool has options for the container parameters and
for lifecycle actions.

The following configuration creates a single pool called "pg" which maintains five containers from the "postgres" image and
exposes port 5432 to clients.  See the [`params`](#params) and [`actions`](#lifecycle-actions) below for documentation on those fields.

```json
{
  "pools": {
    "pg": {
      "image": "postgres",
      "size": 5,
      "port": 5432,
      "params": {
        "username": "postgres",
        "password": "",
        "database": "postgres",
        "url": "postgres://{{.Username}}:{{.Password}}@{{.Hostname}}:{{.Port}}/{{.Database}}?sslmode=disable"
      },
      "actions": {
        "healthcheck": {
          "type": "postgres.ping",
          "retries": 10,
          "delay":   "50ms"
        },
        "initialize": {
          "type":    "exec",
          "path":    "make",
          "args":    ["db:migrate"],
          "env":     ["DATABASE_URL={{.Url}}"],
          "timeout": "10s",
        },
        "reset": {
          "type": "postgres.truncate"
        }
      }
    }
  }
}
```

See [example/config.json](example/config.json) for a full working configuration.

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

```json
{
  "username": "postgres",
  "password": "",
  "database": "postgres",
  "url": "postgres://{{.Username}}:{{.Password}}@{{.Hostname}}:{{.Port}}/{{.Database}}?sslmode=disable"
}
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

There are three lifecycle actions: `healthcheck`, `initialize`, and `reset`.

 * `healthcheck` is used to determine when the container is ready to be used.
 * `initialize` is used to initialize the container (run migrations, etc...)
 * `reset` may be used to revert the container to a state where it can be used again.

All of them are optional (though `healthcheck` should be used).  If `reset` is not given,
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

Execute a command on the host operating system.  Useful for running migrations to initialize a database.

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

```json
{
  "type": "postgres.exec",
  "command": "INSERT INTO users (name) VALUES ($1)",
  "args": "Robert'); DROP TABLE STUDENTS;--"
}
```

#### postgres.ping

Pings the database.  Useful for healthcheck.

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

### TODO

 * Configuration
   * Current parsing is a disaster
   * Allow yaml
   * Allow built-in defaults (postgres, redis, etc...)
 * Polish/Optimize/Cleanup UI.
 * Use simple JSON API instead of [koding/kite](https://github.com/koding/kite).
 * Re-add remote actions
   * Use websockets instead of [koding/kite](https://github.com/koding/kite)
 * Nodejs client example
 * Documentation
 * Tests
