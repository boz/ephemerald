package redis

import (
	"encoding/json"
	"net"
	"strconv"

	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/params"
	rredis "github.com/garyburd/redigo/redis"
)

func init() {
	lifecycle.MakeActionPlugin("redis.exec", actionRedisExecParse)
	lifecycle.MakeActionPlugin("redis.ping", actionRedisExecParse)
}

type actionRedisExec struct {
	lifecycle.ActionConfig
	redisParams
	Command string
}

func actionRedisExecParse(buf []byte) (lifecycle.Generator, error) {
	action := &actionRedisExec{
		ActionConfig: lifecycle.DefaultActionConfig(),
		Command:      "PING",
	}
	return action, parseRedisExec(action, buf)
}

func parseRedisExec(action *actionRedisExec, buf []byte) error {
	err := json.Unmarshal(buf, action)
	if err != nil {
		return err
	}
	return err
}

func (a actionRedisExec) Create() (lifecycle.Action, error) {
	return &a, nil
}

func (a *actionRedisExec) Do(e lifecycle.Env, p params.Params) error {

	p = params.MergeDefaultsWithOverride(p, a.redisParams.overrides(), defaultParamConfig())

	address := net.JoinHostPort(p.Host(), strconv.Itoa(p.Port()))

	dbs, err := p.Var("database")
	if err != nil {
		return err
	}
	db, err := strconv.Atoi(dbs)
	if err != nil {
		return err
	}

	e.Log().WithField("address", address).Debug("dialing")

	conn, err := rredis.Dial("tcp", address,
		rredis.DialConnectTimeout(a.Timeout),
		rredis.DialReadTimeout(a.Timeout),
		rredis.DialWriteTimeout(a.Timeout),
		rredis.DialDatabase(db))

	if err != nil {
		e.Log().WithError(err).Debug("ERROR: dialing")
		return err
	}
	defer conn.Close()

	e.Log().WithField("command", a.Command).Debug("executing")

	// TODO: render command
	_, err = conn.Do(a.Command)
	if err != nil {
		e.Log().WithError(err).Debug("ERROR: executing")
	}
	return err
}
